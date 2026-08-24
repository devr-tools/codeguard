function analyzeDeadCodeReports(programs) {
  if (!input.dead_code || !input.dead_code.enabled) {
    return;
  }
  const reports = activeReportPaths();
  if (reports.length === 0) {
    return;
  }
  const sourceFiles = sourceFilesByRelPath(programs);
  for (const reportPath of reports) {
    const data = readJSONFile(reportPath);
    if (!data) {
      continue;
    }
    if (isEsbuildMetafile(data)) {
      analyzeEsbuildMetafile(data, sourceFiles);
      continue;
    }
    if (isWebpackStats(data)) {
      analyzeWebpackStats(data, sourceFiles);
      continue;
    }
    if (isRollupBundleReport(data)) {
      analyzeRollupBundleReport(data, sourceFiles);
    }
  }
}

function sourceFilesByRelPath(programs) {
  const sourceFiles = new Map();
  for (const program of programs) {
    for (const sourceFile of program.getSourceFiles()) {
      if (!isAnalyzableSourceFile(sourceFile)) {
        continue;
      }
      const relPath = normalizePath(path.relative(targetPath, sourceFile.fileName));
      const flavor = scriptFlavor(relPath);
      if (!flavor || !deadCodeEnabledForFlavor(flavor) || shouldSkipDeadCodePath(relPath, sourceFile.text, flavor)) {
        continue;
      }
      if (!sourceFiles.has(relPath)) {
        sourceFiles.set(relPath, sourceFile);
      }
    }
  }
  return sourceFiles;
}

function activeReportPaths() {
  return resolveConfiguredFiles(activeDeadCodeList("reports"), [".json"], () => true);
}

function readJSONFile(filePath) {
  try {
    return JSON.parse(fs.readFileSync(filePath, "utf8"));
  } catch (error) {
    results.debug.push(`dead_code_report_parse_failed:${normalizePath(path.relative(targetPath, filePath))}`);
    return null;
  }
}

function readOptionalJSONFile(filePath) {
  try {
    if (!ts.sys.fileExists(filePath)) {
      return null;
    }
    return JSON.parse(fs.readFileSync(filePath, "utf8"));
  } catch (error) {
    return null;
  }
}

function isEsbuildMetafile(data) {
  return !!data &&
    typeof data === "object" &&
    data.inputs &&
    typeof data.inputs === "object" &&
    data.outputs &&
    typeof data.outputs === "object";
}

function analyzeEsbuildMetafile(data, sourceFiles) {
  const inputBytesByRelPath = new Map();
  for (const output of Object.values(data.outputs || {})) {
    if (!output || typeof output !== "object" || !output.inputs || typeof output.inputs !== "object") {
      continue;
    }
    for (const [rawInputPath, inputMeta] of Object.entries(output.inputs)) {
      const relPath = normalizeReportSourcePath(rawInputPath);
      if (!relPath || !sourceFiles.has(relPath)) {
        continue;
      }
      const bytes = typeof inputMeta.bytesInOutput === "number" ? inputMeta.bytesInOutput : null;
      if (bytes === null) {
        continue;
      }
      const current = inputBytesByRelPath.get(relPath);
      inputBytesByRelPath.set(relPath, current === undefined ? bytes : Math.max(current, bytes));
    }
  }
  for (const [relPath, maxBytesInOutput] of inputBytesByRelPath.entries()) {
    if (maxBytesInOutput !== 0) {
      continue;
    }
    const sourceFile = sourceFiles.get(relPath);
    const flavor = scriptFlavor(relPath);
    if (!isBundlerDeadCodeCandidate(sourceFile, relPath, { allowExports: true })) {
      continue;
    }
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.dead-code.toolchain",
      input.dead_code.level || "warn",
      `${scriptLabel(flavor)} bundler metafile reports this module was tree-shaken out of every configured output`,
      0,
    );
  }
}

function isWebpackStats(data) {
  return !!data &&
    typeof data === "object" &&
    (Array.isArray(data.modules) || Array.isArray(data.children));
}

function analyzeWebpackStats(data, sourceFiles) {
  for (const moduleInfo of collectWebpackModules(data)) {
    if (!moduleInfo || moduleInfo.orphan !== true) {
      continue;
    }
    const relPath = normalizeWebpackModulePath(moduleInfo);
    if (!relPath || !sourceFiles.has(relPath)) {
      continue;
    }
    const sourceFile = sourceFiles.get(relPath);
    const flavor = scriptFlavor(relPath);
    if (!isBundlerDeadCodeCandidate(sourceFile, relPath, { allowExports: true })) {
      continue;
    }
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.dead-code.toolchain",
      input.dead_code.level || "warn",
      `${scriptLabel(flavor)} webpack stats mark this module as orphaned from emitted chunks`,
      0,
    );
  }
}

function isRollupBundleReport(data) {
  if (!data || typeof data !== "object") {
    return false;
  }
  return collectRollupChunks(data).length > 0;
}

function analyzeRollupBundleReport(data, sourceFiles) {
  const renderedLengthByRelPath = new Map();
  for (const chunk of collectRollupChunks(data)) {
    if (!chunk.modules || typeof chunk.modules !== "object") {
      continue;
    }
    for (const [rawModulePath, moduleInfo] of Object.entries(chunk.modules)) {
      const relPath = normalizeReportSourcePath(rawModulePath);
      if (!relPath || !sourceFiles.has(relPath) || !moduleInfo || typeof moduleInfo !== "object") {
        continue;
      }
      const renderedLength = typeof moduleInfo.renderedLength === "number" ? moduleInfo.renderedLength : null;
      if (renderedLength === null) {
        continue;
      }
      const current = renderedLengthByRelPath.get(relPath);
      renderedLengthByRelPath.set(relPath, current === undefined ? renderedLength : Math.max(current, renderedLength));
    }
  }
  for (const [relPath, maxRenderedLength] of renderedLengthByRelPath.entries()) {
    if (maxRenderedLength !== 0) {
      continue;
    }
    const sourceFile = sourceFiles.get(relPath);
    const flavor = scriptFlavor(relPath);
    if (!isBundlerDeadCodeCandidate(sourceFile, relPath, { allowExports: true })) {
      continue;
    }
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.dead-code.toolchain",
      input.dead_code.level || "warn",
      `${scriptLabel(flavor)} Rollup/Vite artifact reports this module rendered zero bytes in every configured output`,
      0,
    );
  }
}

function collectRollupChunks(value) {
  const chunks = [];
  const visit = (node) => {
    if (!node || typeof node !== "object") {
      return;
    }
    if (Array.isArray(node)) {
      for (const item of node) {
        visit(item);
      }
      return;
    }
    if (node.type === "chunk" && node.modules && typeof node.modules === "object") {
      chunks.push(node);
    }
    for (const key of ["output", "outputs", "chunks", "bundle"]) {
      if (Array.isArray(node[key]) || node[key] && typeof node[key] === "object") {
        visit(node[key]);
      }
    }
  };
  visit(value);
  return chunks;
}

function collectWebpackModules(value) {
  const modules = [];
  function collect(node) {
    if (!node || typeof node !== "object") {
      return;
    }
    if (Array.isArray(node.modules)) {
      modules.push(...node.modules);
      for (const child of node.modules) {
        collect(child);
      }
    }
    if (Array.isArray(node.children)) {
      for (const child of node.children) {
        collect(child);
      }
    }
  }
  collect(value);
  return modules;
}

function normalizeWebpackModulePath(moduleInfo) {
  for (const field of ["identifier", "name", "nameForCondition"]) {
    const relPath = normalizeReportSourcePath(moduleInfo[field]);
    if (relPath) {
      return relPath;
    }
  }
  return "";
}

function isBundlerDeadCodeCandidate(sourceFile, relPath, options) {
  if (isPackageEntrypoint(relPath) || isFrameworkConventionPath(relPath) || isVendorPath(relPath)) {
    return false;
  }
  if (!options.allowExports && hasTopLevelExport(sourceFile)) {
    return false;
  }
  if (hasTopLevelSideEffectStatement(sourceFile)) {
    return false;
  }
  return hasRuntimeDeclaration(sourceFile);
}

function hasTopLevelExport(sourceFile) {
  return sourceFile.statements.some((statement) => {
    const flags = ts.getCombinedModifierFlags(statement);
    return (flags & ts.ModifierFlags.Export) !== 0 || ts.isExportDeclaration(statement) || ts.isExportAssignment(statement);
  });
}

function hasRuntimeDeclaration(sourceFile) {
  return sourceFile.statements.some((statement) =>
    ts.isFunctionDeclaration(statement) ||
    ts.isClassDeclaration(statement) ||
    ts.isEnumDeclaration(statement) ||
    ts.isVariableStatement(statement));
}

function hasTopLevelSideEffectStatement(sourceFile) {
  return sourceFile.statements.some((statement) => {
    if (ts.isImportDeclaration(statement) || ts.isImportEqualsDeclaration(statement)) {
      return !statement.importClause;
    }
    if (ts.isExpressionStatement(statement) || ts.isAwaitExpression(statement)) {
      return true;
    }
    if (ts.isForStatement(statement) || ts.isForInStatement(statement) || ts.isForOfStatement(statement) ||
        ts.isWhileStatement(statement) || ts.isDoStatement(statement) || ts.isIfStatement(statement) ||
        ts.isSwitchStatement(statement) || ts.isTryStatement(statement) || ts.isThrowStatement(statement)) {
      return true;
    }
    return false;
  });
}

function isPackageEntrypoint(relPath) {
  const entrypoints = packageEntrypoints();
  return entrypoints.has(relPath) || entrypoints.has(stripKnownScriptExtension(relPath));
}

let cachedPackageEntrypoints = null;

function packageEntrypoints() {
  if (cachedPackageEntrypoints !== null) {
    return cachedPackageEntrypoints;
  }
  const entrypoints = new Set();
  addConfiguredEntrypoints(entrypoints);
  const pkg = readOptionalJSONFile(path.join(targetPath, "package.json"));
  if (pkg && typeof pkg === "object") {
    addPackageEntrypointValue(entrypoints, pkg.main);
    addPackageEntrypointValue(entrypoints, pkg.module);
    addPackageEntrypointValue(entrypoints, pkg.browser);
    addPackageEntrypointValue(entrypoints, pkg.exports);
  }
  cachedPackageEntrypoints = entrypoints;
  return entrypoints;
}

function addConfiguredEntrypoints(entrypoints) {
  for (const entrypoint of activeDeadCodeList("entrypoints")) {
    const relPath = normalizePath(String(entrypoint || "").trim()).replace(/^\.\//, "");
    if (relPath) {
      entrypoints.add(relPath);
      entrypoints.add(stripKnownScriptExtension(relPath));
    }
  }
}

function addPackageEntrypointValue(entrypoints, value) {
  if (!value) {
    return;
  }
  if (typeof value === "string") {
    const relPath = normalizeReportSourcePath(value);
    if (relPath) {
      entrypoints.add(relPath);
      entrypoints.add(stripKnownScriptExtension(relPath));
    }
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      addPackageEntrypointValue(entrypoints, item);
    }
    return;
  }
  if (typeof value === "object") {
    for (const nested of Object.values(value)) {
      addPackageEntrypointValue(entrypoints, nested);
    }
  }
}

function stripKnownScriptExtension(relPath) {
  for (const suffix of [".d.ts", ".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs", ".mts", ".cts"]) {
    if (relPath.endsWith(suffix)) {
      return relPath.slice(0, -suffix.length);
    }
  }
  return relPath;
}

function isFrameworkConventionPath(relPath) {
  const normalized = relPath.toLowerCase();
  const base = path.basename(normalized);
  if (/^(page|layout|route|loading|error|not-found)\.[cm]?[jt]sx?$/.test(base) &&
      /(^|\/)(app|pages)(\/|$)/.test(normalized)) {
    return true;
  }
  if (/^pages\/api\//.test(normalized) || /^src\/pages\/api\//.test(normalized)) {
    return true;
  }
  return false;
}

function isVendorPath(relPath) {
  const normalized = relPath.toLowerCase();
  return normalized.includes("/node_modules/") ||
    normalized.startsWith("node_modules/") ||
    normalized.includes("/vendor/") ||
    normalized.startsWith("vendor/");
}
