// Realistic benchmark corpus: a plausible dashboard/analytics service module.
// Used by the spike benchmarks; scanned identically by the baseline regexes
// and both tree-sitter engines.

import { EventEmitter } from "node:events";

export interface WorkspaceRef {
  id: string;
  slug: string;
  region: "us" | "eu" | "apac";
}

export interface MetricPoint {
  timestamp: number;
  value: number;
  dimensions: Record<string, string>;
}

export interface MetricSeries {
  name: string;
  unit: string;
  points: MetricPoint[];
}

export interface DashboardTile {
  title: string;
  series: MetricSeries[];
  refreshSeconds: number;
  spanColumns: 1 | 2 | 3 | 4;
}

export interface FetchOptions {
  timeoutMs?: number;
  retries?: number;
  signal?: AbortSignal;
  headers?: Record<string, string>;
}

export type TileState =
  | { kind: "loading" }
  | { kind: "ready"; tile: DashboardTile }
  | { kind: "error"; message: string; retryable: boolean };

export enum RefreshMode {
  Manual = "manual",
  Interval = "interval",
  Realtime = "realtime",
}

const SLUG_PATTERN = /^[a-z0-9][a-z0-9-]{1,62}$/;
const DEFAULT_TIMEOUT_MS = 8000;
const DEFAULT_RETRIES = 2;
const MAX_POINTS_PER_SERIES = 2880;

export class DashboardError extends Error {
  constructor(
    message: string,
    readonly retryable: boolean,
    readonly status?: number,
  ) {
    super(message);
    this.name = "DashboardError";
  }
}

function backoffDelay(attempt: number): number {
  const base = Math.min(1000 * 2 ** attempt, 15000);
  return base / 2 + Math.floor(Math.random() * (base / 2));
}

function joinPath(base: string, ...segments: string[]): string {
  const cleaned = segments.map((segment) => segment.replace(/^\/+|\/+$/g, ""));
  return [base.replace(/\/+$/g, ""), ...cleaned].join("/");
}

export function validateSlug(slug: string): void {
  if (!SLUG_PATTERN.test(slug)) {
    throw new DashboardError(`invalid workspace slug: ${slug}`, false, 400);
  }
}

function normalizeHeaders(headers?: Record<string, string>): Headers {
  const merged = new Headers({ accept: "application/json" });
  for (const [key, value] of Object.entries(headers ?? {})) {
    merged.set(key.toLowerCase(), value);
  }
  return merged;
}

async function fetchJSON<T>(url: string, options: FetchOptions = {}): Promise<T> {
  const timeout = options.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  const retries = options.retries ?? DEFAULT_RETRIES;
  let lastError: unknown = null;

  for (let attempt = 0; attempt <= retries; attempt++) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeout);
    try {
      const response = await fetch(url, {
        headers: normalizeHeaders(options.headers),
        signal: options.signal ?? controller.signal,
      });
      if (response.status === 429 || response.status >= 500) {
        throw new DashboardError(
          `transient upstream failure: ${response.status}`,
          true,
          response.status,
        );
      }
      if (!response.ok) {
        throw new DashboardError(
          `request failed: ${response.status} ${response.statusText}`,
          false,
          response.status,
        );
      }
      return (await response.json()) as T;
    } catch (error) {
      lastError = error;
      const retryable = error instanceof DashboardError ? error.retryable : true;
      if (!retryable || attempt === retries) {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, backoffDelay(attempt)));
    } finally {
      clearTimeout(timer);
    }
  }
  throw lastError instanceof Error
    ? lastError
    : new DashboardError("exhausted retries", false);
}

function decodeSeries(payload: unknown): MetricSeries[] {
  // The ingest API predates our schema types, so the decoder starts from a
  // dynamic payload and narrows field by field.
  const body = payload as any; // upstream response has no published schema
  if (!Array.isArray(body?.series)) {
    return [];
  }
  const out: MetricSeries[] = [];
  for (const raw of body.series) {
    if (typeof raw?.name !== "string" || !Array.isArray(raw?.points)) {
      continue;
    }
    const points: MetricPoint[] = [];
    for (const point of raw.points.slice(0, MAX_POINTS_PER_SERIES)) {
      const timestamp = Number(point?.t);
      const value = Number(point?.v);
      if (!Number.isFinite(timestamp) || !Number.isFinite(value)) {
        continue;
      }
      points.push({
        timestamp,
        value,
        dimensions: typeof point?.d === "object" && point.d ? point.d : {},
      });
    }
    out.push({ name: raw.name, unit: typeof raw.unit === "string" ? raw.unit : "count", points });
  }
  return out;
}

export function aggregate(points: MetricPoint[], bucketSeconds: number): MetricPoint[] {
  if (bucketSeconds <= 0) {
    throw new DashboardError(`bucketSeconds must be positive, got ${bucketSeconds}`, false);
  }
  const buckets = new Map<number, { sum: number; count: number; dims: Record<string, string> }>();
  for (const point of points) {
    const key = Math.floor(point.timestamp / bucketSeconds) * bucketSeconds;
    const bucket = buckets.get(key) ?? { sum: 0, count: 0, dims: point.dimensions };
    bucket.sum += point.value;
    bucket.count += 1;
    buckets.set(key, bucket);
  }
  return [...buckets.entries()]
    .sort(([a], [b]) => a - b)
    .map(([timestamp, bucket]) => ({
      timestamp,
      value: bucket.sum / bucket.count,
      dimensions: bucket.dims,
    }));
}

export function percentile(values: number[], q: number): number {
  if (values.length === 0) {
    return Number.NaN;
  }
  const sorted = [...values].sort((a, b) => a - b);
  const rank = (q / 100) * (sorted.length - 1);
  const low = Math.floor(rank);
  const high = Math.ceil(rank);
  if (low === high) {
    return sorted[low];
  }
  return sorted[low] + (sorted[high] - sorted[low]) * (rank - low);
}

interface CacheEntry<T> {
  value: T;
  expiresAt: number;
}

class TTLCache<T> {
  private readonly entries = new Map<string, CacheEntry<T>>();

  constructor(private readonly ttlMs: number) {}

  get(key: string): T | undefined {
    const entry = this.entries.get(key);
    if (!entry) {
      return undefined;
    }
    if (entry.expiresAt < Date.now()) {
      this.entries.delete(key);
      return undefined;
    }
    return entry.value;
  }

  set(key: string, value: T): void {
    this.entries.set(key, { value, expiresAt: Date.now() + this.ttlMs });
  }

  clear(): void {
    this.entries.clear();
  }
}

export interface DashboardClientConfig {
  baseUrl: string;
  workspace: WorkspaceRef;
  refreshMode: RefreshMode;
  cacheTtlMs?: number;
}

export class DashboardClient extends EventEmitter {
  private readonly cache: TTLCache<MetricSeries[]>;
  private readonly states = new Map<string, TileState>();
  private disposed = false;

  constructor(private readonly config: DashboardClientConfig) {
    super();
    validateSlug(config.workspace.slug);
    this.cache = new TTLCache(config.cacheTtlMs ?? 30_000);
  }

  tileState(title: string): TileState {
    return this.states.get(title) ?? { kind: "loading" };
  }

  private endpoint(metric: string): string {
    const { baseUrl, workspace } = this.config;
    return joinPath(baseUrl, "v1", workspace.region, workspace.id, "metrics", metric);
  }

  async loadSeries(metric: string, options?: FetchOptions): Promise<MetricSeries[]> {
    if (this.disposed) {
      throw new DashboardError("client disposed", false);
    }
    const cached = this.cache.get(metric);
    if (cached) {
      return cached;
    }
    const payload = await fetchJSON<unknown>(this.endpoint(metric), options);
    const series = decodeSeries(payload);
    this.cache.set(metric, series);
    this.emit("series", metric, series.length);
    return series;
  }

  async loadTile(tile: DashboardTile, options?: FetchOptions): Promise<TileState> {
    this.states.set(tile.title, { kind: "loading" });
    try {
      const loaded = await Promise.all(
        tile.series.map((series) => this.loadSeries(series.name, options)),
      );
      const hydrated: DashboardTile = {
        ...tile,
        series: loaded.flat(),
      };
      const state: TileState = { kind: "ready", tile: hydrated };
      this.states.set(tile.title, state);
      return state;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      const retryable = error instanceof DashboardError ? error.retryable : false;
      const state: TileState = { kind: "error", message, retryable };
      this.states.set(tile.title, state);
      this.emit("tile-error", tile.title, message);
      return state;
    }
  }

  dispose(): void {
    this.disposed = true;
    this.cache.clear();
    this.removeAllListeners();
  }
}

function formatValue(value: number, unit: string): string {
  if (!Number.isFinite(value)) {
    return "–";
  }
  const rounded = Math.abs(value) >= 100 ? value.toFixed(0) : value.toFixed(2);
  return `${rounded} ${unit}`;
}

function escapeHTML(text: string): string {
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function tileMarkup(tile: DashboardTile): string {
  const rows = tile.series
    .map((series) => {
      const latest = series.points.at(-1);
      const display = latest ? formatValue(latest.value, series.unit) : "no data";
      return `<tr><td>${escapeHTML(series.name)}</td><td>${escapeHTML(display)}</td></tr>`;
    })
    .join("");
  return `<table class="tile" data-span="${tile.spanColumns}">${rows}</table>`;
}

export function renderTile(container: HTMLElement, tile: DashboardTile): void {
  // Values are escaped above; the sink is still flagged for review.
  container.innerHTML = tileMarkup(tile);
}

export function renderLegend(container: HTMLElement, tiles: DashboardTile[]): void {
  const labels = tiles.map((tile) => tile.title).map(escapeHTML);
  const items = labels.map((label) => `<li>${label}</li>`).join("");
  container.innerHTML = `<ul class="legend">${items}</ul>`;
}

export function summarize(tiles: DashboardTile[]): Record<string, number> {
  const summary: Record<string, number> = {};
  for (const tile of tiles) {
    for (const series of tile.series) {
      const values = series.points.map((point) => point.value);
      summary[`${tile.title}:${series.name}:p95`] = percentile(values, 95);
      summary[`${tile.title}:${series.name}:p50`] = percentile(values, 50);
    }
  }
  return summary;
}

// Legacy bridge: the embedding shell still calls this with untyped tiles.
export function hydrateLegacyTiles(client: DashboardClient, tiles: any[]): Promise<TileState[]> {
  return Promise.all(tiles.map((tile) => client.loadTile(tile as DashboardTile)));
}
