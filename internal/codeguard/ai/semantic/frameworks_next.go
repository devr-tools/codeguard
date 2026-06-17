package semantic

import (
	"path/filepath"
	"strings"
)

func nextJSSignals(file FileSnapshot) []string {
	lowerPath := strings.ToLower(filepath.ToSlash(file.Path))
	content := file.Content
	signals := make([]string, 0, 5)
	if (strings.HasPrefix(lowerPath, "app/") || strings.Contains(lowerPath, "/app/")) && hasAnySuffix(lowerPath, "/route.ts", "/route.tsx", "/route.js", "/route.jsx") {
		signals = append(signals, "app-router-route-file")
	}
	if strings.HasPrefix(lowerPath, "pages/api/") || strings.Contains(lowerPath, "/pages/api/") {
		signals = append(signals, "pages-api-route-file")
	}
	if containsAny(content, `from "next/server"`, "from 'next/server'") {
		signals = append(signals, "next-server-import")
	}
	if (strings.HasPrefix(lowerPath, "app/") || strings.Contains(lowerPath, "/app/")) && hasAnySuffix(lowerPath, "/page.tsx", "/page.jsx", "/layout.tsx", "/layout.jsx", "/loading.tsx", "/loading.jsx", "/error.tsx", "/error.jsx") {
		signals = append(signals, "app-router-component-file")
	}
	if containsAny(content, `"use client"`, `'use client'`) {
		signals = append(signals, "use-client-directive")
	}
	if containsAny(content, "NextRequest", "NextResponse") {
		signals = append(signals, "next-request-response")
	}
	if containsAny(content, "export async function GET", "export async function POST", "export async function PUT", "export async function PATCH", "export async function DELETE", "export function GET", "export function POST", "export function PUT", "export function PATCH", "export function DELETE") {
		signals = append(signals, "route-handler-export")
	}
	return uniqueSortedStrings(signals)
}

func nextJSHints(file FileSnapshot) []string {
	lowerPath := strings.ToLower(filepath.ToSlash(file.Path))
	content := file.Content
	hints := make([]string, 0, 5)
	if containsAny(content, "export async function GET", "export async function POST", "export async function PUT", "export async function PATCH", "export async function DELETE", "export function GET", "export function POST", "export function PUT", "export function PATCH", "export function DELETE") {
		hints = append(hints, "route-handler-contract")
	}
	if (strings.HasPrefix(lowerPath, "app/") || strings.Contains(lowerPath, "/app/")) && hasAnySuffix(lowerPath, "/page.tsx", "/page.jsx", "/layout.tsx", "/layout.jsx", "/loading.tsx", "/loading.jsx", "/error.tsx", "/error.jsx") {
		hints = append(hints, "route-segment-component")
		if containsAny(content, `"use client"`, `'use client'`) {
			hints = append(hints, "client-component")
		} else {
			hints = append(hints, "server-component")
		}
	}
	if containsAny(content, "params", "searchParams") {
		hints = append(hints, "route-props-contract")
	}
	if containsAny(content, "export async function", "export default async function") {
		hints = append(hints, "async-data-contract")
	}
	return uniqueSortedStrings(hints)
}
