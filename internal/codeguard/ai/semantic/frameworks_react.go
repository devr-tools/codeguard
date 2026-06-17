package semantic

import (
	"path/filepath"
	"strings"
)

func reactSignals(file FileSnapshot) []string {
	lowerPath := strings.ToLower(filepath.ToSlash(file.Path))
	content := file.Content
	signals := make([]string, 0, 6)
	if !hasAnySuffix(lowerPath, ".tsx", ".jsx") {
		return nil
	}
	if containsAny(content, `from "react"`, "from 'react'", `from "next/link"`, `from "next/navigation"`, `from "next/image"`, `from "next/head"`) {
		signals = append(signals, "react-import")
	}
	if containsAny(content, "return (", "return<", "</", "/>", "<div", "<main", "<section", "<button", "<form") {
		signals = append(signals, "jsx-component")
	}
	if containsAny(content, "export default function ", "export function ", "const ", "function ") && containsComponentExport(content) {
		signals = append(signals, "component-export")
	}
	if containsAny(content, `"use client"`, `'use client'`) {
		signals = append(signals, "use-client-directive")
	}
	if containsAny(content, "useState(", "useEffect(", "useReducer(", "useTransition(", "useDeferredValue(") {
		signals = append(signals, "react-hooks")
	}
	return uniqueSortedStrings(signals)
}

func reactHints(file FileSnapshot) []string {
	content := file.Content
	hints := make([]string, 0, 4)
	if containsAny(content, "(props", "({", ": Props", "interface Props", "type Props") {
		hints = append(hints, "component-props-contract")
	}
	if containsAny(content, "useState(", "useEffect(", "useReducer(", "useTransition(", "useDeferredValue(") {
		hints = append(hints, "stateful-component")
	}
	if containsAny(content, `"use client"`, `'use client'`) {
		hints = append(hints, "client-component")
	}
	if containsAny(content, "children", "ReactNode", "PropsWithChildren") {
		hints = append(hints, "children-slot-contract")
	}
	return uniqueSortedStrings(hints)
}
