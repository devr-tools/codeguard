package semantic

import "strings"

func expressSignals(file FileSnapshot) []string {
	content := file.Content
	signals := make([]string, 0, 4)
	if containsAny(content, `from "express"`, "from 'express'", `require("express")`, `require('express')`) {
		signals = append(signals, "express-import")
	}
	if strings.Contains(content, "express.Router(") || strings.Contains(content, ".Router(") || strings.Contains(content, "Router()") {
		signals = append(signals, "express-router")
	}
	if containsAny(content, ".get(", ".post(", ".put(", ".patch(", ".delete(", ".use(") {
		signals = append(signals, "http-route-handler")
	}
	return uniqueSortedStrings(signals)
}

func expressHints(file FileSnapshot) []string {
	content := file.Content
	hints := make([]string, 0, 4)
	if containsAny(content, ".use(", "next()", "next)") {
		hints = append(hints, "middleware-order-sensitive")
	}
	if containsAny(content, "next: NextFunction", " next)", ", next)", "(req, res, next)", "(request, response, next)") {
		hints = append(hints, "middleware-next-chain")
	}
	if containsAny(content, "req.", "request.", ": Request") {
		hints = append(hints, "request-derived-contract")
	}
	if containsAny(content, "res.", "response.", "Response") {
		hints = append(hints, "response-side-effects")
	}
	return uniqueSortedStrings(hints)
}
