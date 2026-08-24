const fs = require("fs");
const path = require("path");

const input = JSON.parse(fs.readFileSync(0, "utf8"));
const ts = require(input.typescript_lib_path);
const targetPath = path.resolve(input.target_path);

const results = { design: [], quality: [], security: [], debug: [] };
const seen = new Set();
const taintModel = normalizeTaintModel(input.taint_model);

const directivePatterns = [
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-ignore\b/, suffix: "ts-ignore", message: "suppression comment should be reviewed" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-nocheck\b/, suffix: "ts-nocheck", message: "file-level type checking is disabled" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-expect-error\b/, suffix: "ts-expect-error", message: "suppression comment should be reviewed" },
];

function main() {
  const programs = loadPrograms();
  for (const program of programs) {
    const checker = program.getTypeChecker();

    analyzeDeadCodeDiagnostics(program);

    for (const sourceFile of program.getSourceFiles()) {
      if (!isAnalyzableSourceFile(sourceFile)) {
        continue;
      }
      const relPath = normalizePath(path.relative(targetPath, sourceFile.fileName));
      const flavor = scriptFlavor(relPath);
      if (!flavor) {
        continue;
      }

      analyzeModuleName(sourceFile, relPath);
      analyzeDirectives(sourceFile, relPath, flavor);
      analyzeDesign(sourceFile, relPath);

      const bindings = collectBindings(sourceFile);
      sourceFile.bindings = bindings;
      analyzeTaintFlows(sourceFile, relPath, flavor, bindings, checker);
      visit(sourceFile, sourceFile, relPath, flavor, bindings, checker);
    }

    analyzeTaint(program, checker);
  }

  analyzeDeadCodeReports(programs);

  process.stdout.write(JSON.stringify(results));
}

function loadPrograms() {
  const configPaths = selectedConfigPaths();
  if (configPaths.length > 0) {
    return configPaths.map(loadProgramFromConfig);
  }
  return [loadProgramWithoutConfig()];
}

function loadProgramFromConfig(configPath) {
  const config = ts.readConfigFile(configPath, ts.sys.readFile);
  if (config.error) {
    throw new Error(ts.flattenDiagnosticMessageText(config.error.messageText, "\n"));
  }
  const parsed = ts.parseJsonConfigFileContent(
    config.config,
    ts.sys,
    path.dirname(configPath),
    defaultCompilerOptions(),
    configPath,
  );
  const options = parsed.options;
  applyDeadCodeCompilerOptions(options);
  return ts.createProgram({
    rootNames: sourceRootsForConfig(parsed),
    options,
  });
}

function loadProgramWithoutConfig() {
  const options = defaultCompilerOptions();
  applyDeadCodeCompilerOptions(options);
  return ts.createProgram({
    rootNames: input.source_files || ts.sys.readDirectory(targetPath, scriptExtensions(), undefined, undefined),
    options,
  });
}

function sourceRootsForConfig(parsed) {
  const configured = parsed.fileNames.filter((name) => isWithinTarget(path.resolve(name)));
  const allowed = sourceFileSet();
  if (!allowed) {
    return configured;
  }
  const roots = new Map();
  for (const name of configured) {
    if (allowed.has(path.resolve(name))) {
      roots.set(path.resolve(name), name);
    }
  }
  for (const name of input.source_files) {
    const absolute = path.resolve(name);
    if (isWithinTarget(absolute)) {
      roots.set(absolute, name);
    }
  }
  return Array.from(roots.values()).sort();
}

function sourceFileSet() {
  if (!Array.isArray(input.source_files)) {
    return null;
  }
  return new Set(input.source_files.map((name) => path.resolve(name)));
}

function selectedConfigPaths() {
  const configured = configuredProjectPaths();
  if (configured.length > 0) {
    return configured;
  }
  const configPath = findConfigPath();
  return configPath ? [configPath] : [];
}

function configuredProjectPaths() {
  if (!input.dead_code || !input.dead_code.enabled) {
    return [];
  }
  const patterns = activeDeadCodeList("projects");
  return resolveConfiguredFiles(patterns, [".json"], isProjectConfigFile);
}

function findConfigPath() {
  return ts.findConfigFile(targetPath, ts.sys.fileExists, "tsconfig.json") ||
    ts.findConfigFile(targetPath, ts.sys.fileExists, "jsconfig.json");
}

function activeDeadCodeList(kind) {
  if (!input.dead_code) {
    return [];
  }
  const language = String(input.target_language || "").toLowerCase();
  if (language === "javascript" || language === "js" || language === "jsx") {
    return input.dead_code[`javascript_${kind}`] || [];
  }
  return input.dead_code[`typescript_${kind}`] || [];
}

function resolveConfiguredFiles(patterns, extensions, predicate) {
  if (!Array.isArray(patterns) || patterns.length === 0) {
    return [];
  }
  const resolved = new Set();
  for (const pattern of patterns) {
    const normalized = normalizePath(String(pattern || "").trim()).replace(/^\.\//, "");
    if (!normalized) {
      continue;
    }
    if (!normalized.includes("*")) {
      const candidate = path.resolve(targetPath, normalized);
      if (isWithinTarget(candidate) && ts.sys.fileExists(candidate) && predicate(candidate)) {
        resolved.add(candidate);
      }
      continue;
    }
    const candidates = ts.sys.readDirectory(targetPath, extensions, undefined, undefined);
    for (const candidate of candidates) {
      const relPath = normalizePath(path.relative(targetPath, candidate));
      if (matchesPathPattern(relPath, normalized) && predicate(candidate)) {
        resolved.add(path.resolve(candidate));
      }
    }
  }
  return Array.from(resolved).sort();
}

function isProjectConfigFile(filePath) {
  const base = path.basename(filePath).toLowerCase();
  return base === "tsconfig.json" || base === "jsconfig.json" || /^tsconfig\..+\.json$/.test(base) || /^jsconfig\..+\.json$/.test(base);
}

function defaultCompilerOptions() {
  return {
    allowJs: true,
    checkJs: true,
    noEmit: true,
    skipLibCheck: true,
    target: ts.ScriptTarget.Latest,
    module: ts.ModuleKind.ESNext,
    jsx: ts.JsxEmit.Preserve,
  };
}

function applyDeadCodeCompilerOptions(options) {
  if (!input.dead_code || !input.dead_code.enabled) {
    return;
  }
  options.noUnusedLocals = true;
  options.noUnusedParameters = true;
  options.allowUnreachableCode = false;
}

function scriptExtensions() {
  return [".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"];
}

function isAnalyzableSourceFile(sourceFile) {
  return !sourceFile.isDeclarationFile &&
    scriptFlavor(sourceFile.fileName) &&
    isWithinTarget(sourceFile.fileName);
}

function isWithinTarget(fileName) {
  const resolved = path.resolve(fileName);
  return resolved === targetPath || resolved.startsWith(targetPath + path.sep);
}

function normalizePath(value) {
  return value.split(path.sep).join("/");
}

function normalizeReportSourcePath(value) {
  if (typeof value !== "string") {
    return "";
  }
  let normalized = value.trim();
  if (!normalized) {
    return "";
  }
  normalized = normalized.split("!").pop();
  normalized = normalized.split("?")[0].split("#")[0];
  normalized = normalized.replace(/^webpack:\/\/[^/]+\//, "");
  normalized = normalized.replace(/^\.\//, "");
  const dotSlash = normalized.indexOf("/./");
  if (dotSlash >= 0) {
    normalized = normalized.slice(dotSlash + 3);
  }
  normalized = normalizePath(normalized);
  if (path.isAbsolute(normalized)) {
    if (!isWithinTarget(normalized)) {
      return "";
    }
    normalized = normalizePath(path.relative(targetPath, normalized));
  }
  normalized = normalized.replace(/^\.\//, "");
  if (!scriptFlavor(normalized) || normalized.startsWith("../") || isVendorPath(normalized)) {
    return "";
  }
  return normalized;
}

function scriptFlavor(fileName) {
  switch (path.extname(fileName).toLowerCase()) {
    case ".ts":
    case ".tsx":
    case ".mts":
    case ".cts":
      return "typescript";
    case ".js":
    case ".jsx":
    case ".mjs":
    case ".cjs":
      return "javascript";
    default:
      return "";
  }
}

function scriptLabel(flavor) {
  return flavor === "javascript" ? "JavaScript" : "TypeScript";
}

function scriptRuleId(flavor, tsRuleId, jsRuleId) {
  return flavor === "javascript" ? jsRuleId : tsRuleId;
}

function lineNumber(sourceFile, pos) {
  return sourceFile.getLineAndCharacterOfPosition(pos).line + 1;
}

function normalizeTaintModel(model) {
  const normalized = { sources: [], sinks: [] };
  if (!model || typeof model !== "object") {
    return normalized;
  }
  normalized.sources = Array.isArray(model.sources) ? model.sources : [];
  normalized.sinks = Array.isArray(model.sinks) ? model.sinks : [];
  return normalized;
}

function pushFinding(section, sourceFile, relPath, flavor, ruleId, level, message, pos) {
  const line = lineNumber(sourceFile, pos);
  const key = [section, ruleId, relPath, line, message].join("|");
  if (seen.has(key)) {
    return;
  }
  seen.add(key);
  results[section].push({
    rule_id: ruleId,
    level,
    path: relPath,
    line,
    column: 1,
    message,
  });
}

function analyzeDeadCodeDiagnostics(program) {
  if (!input.dead_code || !input.dead_code.enabled) {
    return;
  }
  const diagnostics = program.getSemanticDiagnostics();
  for (const diagnostic of diagnostics) {
    if (!isDeadCodeDiagnostic(diagnostic) || !diagnostic.file) {
      continue;
    }
    const sourceFile = diagnostic.file;
    if (!isAnalyzableSourceFile(sourceFile)) {
      continue;
    }
    const relPath = normalizePath(path.relative(targetPath, sourceFile.fileName));
    const flavor = scriptFlavor(relPath);
    if (!flavor || !deadCodeEnabledForFlavor(flavor) || shouldSkipDeadCodePath(relPath, sourceFile.text, flavor)) {
      continue;
    }
    const message = ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n");
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.dead-code.toolchain",
      input.dead_code.level || "warn",
      `${scriptLabel(flavor)} compiler reports dead or unused code: ${message}`,
      typeof diagnostic.start === "number" ? diagnostic.start : 0,
    );
  }
}

function isDeadCodeDiagnostic(diagnostic) {
  switch (diagnostic.code) {
    case 6133: // declared but never read
    case 6192: // all imports in import declaration are unused
    case 6196: // declared but never used
    case 7027: // unreachable code detected
      return true;
    default:
      return false;
  }
}

function deadCodeEnabledForFlavor(flavor) {
  const language = String(input.target_language || "").toLowerCase();
  if (flavor === "javascript") {
    return language === "javascript" || language === "js" || language === "jsx";
  }
  return language === "typescript" || language === "ts" || language === "tsx";
}

function shouldSkipDeadCodePath(relPath, sourceText, flavor) {
  if (matchesAnyPathPattern(relPath, flavor === "javascript" ? input.dead_code.javascript_ignore_paths : input.dead_code.typescript_ignore_paths)) {
    return true;
  }
  if (!input.dead_code.include_tests && isTestOrStoryPath(relPath)) {
    return true;
  }
  if (isGeneratedSource(relPath, sourceText)) {
    return true;
  }
  return false;
}

function isTestOrStoryPath(relPath) {
  const lower = relPath.toLowerCase();
  const base = path.basename(lower);
  return lower.startsWith("__tests__/") ||
    lower.startsWith("test/") ||
    lower.startsWith("tests/") ||
    lower.startsWith("fixtures/") ||
    lower.startsWith("fixture/") ||
    lower.includes("/__tests__/") ||
    lower.includes("/test/") ||
    lower.includes("/tests/") ||
    lower.includes("/fixtures/") ||
    lower.includes("/fixture/") ||
    base.includes(".test.") ||
    base.includes(".spec.") ||
    base.includes(".stories.") ||
    base.includes(".story.");
}

function isGeneratedSource(relPath, sourceText) {
  const lower = relPath.toLowerCase();
  if (lower.includes("/generated/") || lower.includes("/__generated__/") || lower.endsWith(".generated.ts") || lower.endsWith(".generated.tsx") || lower.endsWith(".generated.js") || lower.endsWith(".generated.jsx")) {
    return true;
  }
  const header = sourceText.slice(0, 4096);
  return /code generated/i.test(header) && /do not edit/i.test(header);
}

function matchesAnyPathPattern(relPath, patterns) {
  if (!Array.isArray(patterns)) {
    return false;
  }
  for (const pattern of patterns) {
    if (matchesPathPattern(relPath, String(pattern || ""))) {
      return true;
    }
  }
  return false;
}

function matchesPathPattern(relPath, pattern) {
  const normalizedPattern = normalizePath(pattern.trim()).replace(/\\/g, "/");
  if (!normalizedPattern) {
    return false;
  }
  if (normalizedPattern === relPath) {
    return true;
  }
  if (normalizedPattern.endsWith("/**")) {
    const prefix = normalizedPattern.slice(0, -3);
    return relPath === prefix || relPath.startsWith(prefix + "/");
  }
  if (normalizedPattern.includes("*")) {
    const regex = new RegExp("^" + globPatternToRegExp(normalizedPattern) + "$");
    return regex.test(relPath);
  }
  return relPath.startsWith(normalizedPattern.endsWith("/") ? normalizedPattern : normalizedPattern + "/");
}

function globPatternToRegExp(pattern) {
  let out = "";
  for (let index = 0; index < pattern.length; index++) {
    const char = pattern[index];
    if (char !== "*") {
      out += escapeRegExp(char);
      continue;
    }
    if (pattern[index + 1] === "*") {
      out += ".*";
      index++;
      continue;
    }
    out += "[^/]*";
  }
  return out;
}

function escapeRegExp(value) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function analyzeModuleName(sourceFile, relPath) {
  const moduleName = normalizedModuleName(relPath);
  for (const forbidden of input.forbidden_package_names || []) {
    if (moduleName !== String(forbidden || "").toLowerCase()) {
      continue;
    }
    pushFinding(
      "design",
      sourceFile,
      relPath,
      scriptFlavor(relPath),
      "design.typescript.generic-module-name",
      "warn",
      `module name "${moduleName}" is too generic`,
      0,
    );
  }
}

function normalizedModuleName(relPath) {
  const lower = path.basename(relPath).toLowerCase();
  for (const ext of [".d.ts", ".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs", ".mts", ".cts"]) {
    if (lower.endsWith(ext)) {
      return lower.slice(0, -ext.length);
    }
  }
  return lower.slice(0, -path.extname(lower).length);
}


function analyzeDirectives(sourceFile, relPath, flavor) {
  const lines = sourceFile.text.replace(/\r\n/g, "\n").split("\n");
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index];
    for (const directive of directivePatterns) {
      if (!directive.pattern.test(line)) {
        continue;
      }
      pushFinding(
        "quality",
        sourceFile,
        relPath,
        flavor,
        scriptRuleId(flavor, `quality.typescript.${directive.suffix}`, `quality.javascript.${directive.suffix}`),
        "warn",
        `${scriptLabel(flavor)} ${directive.message}`,
        sourceFile.getPositionOfLineAndCharacter(index, 0),
      );
    }
  }
}

function analyzeDesign(sourceFile, relPath) {
  const flavor = scriptFlavor(relPath);
  ts.forEachChild(sourceFile, (node) => {
    if (ts.isClassDeclaration(node) || ts.isClassExpression(node)) {
      const name = classLikeName(node);
      const methods = node.members.filter(isCountedClassMember).length;
      if (methods > input.max_methods_per_type) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-methods-per-type",
          "warn",
          `class ${name} has ${methods} methods; max is ${input.max_methods_per_type}`,
          node.name ? node.name.getStart(sourceFile) : node.getStart(sourceFile),
        );
      }
    }

    if (ts.isInterfaceDeclaration(node)) {
      const members = node.members.length;
      if (members > input.max_interface_members && !isAllowedLargeDataShape(node.name.text, relPath)) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-interface-members",
          "warn",
          `interface ${node.name.text} has ${members} members; max is ${input.max_interface_members}`,
          node.name.getStart(sourceFile),
        );
      }
    }

    if (ts.isTypeAliasDeclaration(node) && ts.isTypeLiteralNode(node.type)) {
      const members = node.type.members.length;
      if (members > input.max_interface_members && !isAllowedLargeDataShape(node.name.text, relPath)) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-interface-members",
          "warn",
          `type ${node.name.text} has ${members} members; max is ${input.max_interface_members}`,
          node.name.getStart(sourceFile),
        );
      }
    }
  });
}

function isAllowedLargeDataShape(name, relPath) {
  const normalizedName = String(name || "").toLowerCase();
  const normalizedPath = String(relPath || "").replace(/\\/g, "/").toLowerCase();
  if (/(props|row|rows|config|definition|field|fields|dto|record|input|output|response|payload|schema|theme)$/.test(normalizedName)) {
    return true;
  }
  if (/(^|\/)(__fixtures__|fixtures|testdata|types|schemas|dto|config|db|database|prisma)(\/|$)/.test(normalizedPath)) {
    return true;
  }
  if (/\.(test|spec)\.[tj]sx?$/.test(normalizedPath)) {
    return true;
  }
  return false;
}

function classLikeName(node) {
  return node.name && node.name.text ? node.name.text : "anonymous";
}

function isCountedClassMember(member) {
  return (ts.isMethodDeclaration(member) || ts.isGetAccessorDeclaration(member) || ts.isSetAccessorDeclaration(member)) &&
      member.name &&
      !ts.isConstructorDeclaration(member) ||
    ts.isPropertyDeclaration(member) &&
      member.initializer &&
      (ts.isArrowFunction(member.initializer) || ts.isFunctionExpression(member.initializer));
}

function visit(node, sourceFile, relPath, flavor, bindings, checker) {
  analyzeQualityNode(node, sourceFile, relPath, flavor);
  analyzeSecurityNode(node, sourceFile, relPath, flavor, bindings, checker);
  analyzeFunctionMetrics(node, sourceFile, relPath, flavor);
  ts.forEachChild(node, (child) => visit(child, sourceFile, relPath, flavor, bindings, checker));
}

function analyzeQualityNode(node, sourceFile, relPath, flavor) {
  if (ts.isDebuggerStatement(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.debugger-statement", "quality.javascript.debugger-statement"),
      "warn",
      "debugger statements should not reach committed source",
      node.getStart(sourceFile),
    );
  }

  if (node.kind === ts.SyntaxKind.AnyKeyword) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.explicit-any", "quality.javascript.explicit-any"),
      "warn",
      "explicit any should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (isDoubleAssertion(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.double-assertion", "quality.javascript.double-assertion"),
      "warn",
      "double type assertions should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (ts.isNonNullExpression(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.non-null-assertion", "quality.javascript.non-null-assertion"),
      "warn",
      "non-null assertions should be reviewed",
      node.getStart(sourceFile),
    );
  }
}

function isDoubleAssertion(node) {
  return isAssertionExpression(node) &&
    isAssertionExpression(node.expression) &&
    isAnyOrUnknownType(node.expression.type);
}

function isAssertionExpression(node) {
  return ts.isAsExpression(node) || ts.isTypeAssertionExpression(node);
}

function isAnyOrUnknownType(node) {
  return !!node && (node.kind === ts.SyntaxKind.AnyKeyword || node.kind === ts.SyntaxKind.UnknownKeyword);
}

function analyzeFunctionMetrics(node, sourceFile, relPath, flavor) {
  if (!isMeasuredFunction(node)) {
    return;
  }

  const metrics = functionMetrics(node, sourceFile);
  if (metrics.length > input.max_function_lines) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.max-function-lines",
      "warn",
      `function ${metrics.name} has ${metrics.length} lines; max is ${input.max_function_lines}`,
      metrics.pos,
    );
  }
  if (metrics.params > input.max_parameters) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.max-parameters",
      "warn",
      `function ${metrics.name} has ${metrics.params} parameters; max is ${input.max_parameters}`,
      metrics.pos,
    );
  }
  if (metrics.complexity > input.max_cyclomatic_complexity) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.cyclomatic-complexity",
      "warn",
      `function ${metrics.name} has cyclomatic complexity ${metrics.complexity}; max is ${input.max_cyclomatic_complexity}`,
      metrics.pos,
    );
  }
}

function isMeasuredFunction(node) {
  if (!ts.isFunctionLike(node) || ts.isConstructorDeclaration(node) || !node.body) {
    return false;
  }
  return ts.isArrowFunction(node) ||
    ts.isFunctionDeclaration(node) ||
    ts.isFunctionExpression(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node);
}

function functionMetrics(node, sourceFile) {
  const startLine = lineNumber(sourceFile, node.getStart(sourceFile));
  const endLine = lineNumber(sourceFile, node.body.end);
  return {
    name: functionName(node),
    pos: node.name ? node.name.getStart(sourceFile) : node.getStart(sourceFile),
    length: Math.max(1, endLine - startLine + 1),
    params: node.parameters.length,
    complexity: functionComplexity(node.body, node),
  };
}

function functionName(node) {
  if (node.name && ts.isIdentifier(node.name)) {
    return node.name.text;
  }
  if (ts.isVariableDeclaration(node.parent) && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }
  if (ts.isPropertyDeclaration(node.parent) && node.parent.name && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }
  return "anonymous";
}

function functionComplexity(body, root) {
  let complexity = 1;

  function walk(node) {
    if (node !== root && ts.isFunctionLike(node)) {
      return;
    }
    if (incrementsComplexity(node)) {
      complexity++;
    }
    ts.forEachChild(node, walk);
  }

  walk(body);
  return complexity;
}

function incrementsComplexity(node) {
  return ts.isIfStatement(node) ||
    ts.isForStatement(node) ||
    ts.isForInStatement(node) ||
    ts.isForOfStatement(node) ||
    ts.isWhileStatement(node) ||
    ts.isDoStatement(node) ||
    ts.isCatchClause(node) ||
    ts.isConditionalExpression(node) ||
    ts.isCaseClause(node) ||
    isShortCircuitOperator(node);
}

function isShortCircuitOperator(node) {
  return ts.isBinaryExpression(node) &&
    (node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
      node.operatorToken.kind === ts.SyntaxKind.BarBarToken ||
      node.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken);
}

function analyzeTaintFlows(sourceFile, relPath, flavor, bindings, checker) {
  if (taintModel.sources.length === 0 || taintModel.sinks.length === 0) {
    return;
  }
  const symbolTaintCache = new Map();
  const expressionTaintCache = new Map();
  visitTaintNode(sourceFile);

  function visitTaintNode(node) {
    if (ts.isCallExpression(node)) {
      const sink = taintSinkForCall(node, bindings, checker);
      if (sink && callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.expression.getStart(sourceFile),
        );
      }
    }

    if (ts.isNewExpression(node)) {
      const sink = taintSinkForNewExpression(node);
      if (sink && callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.expression.getStart(sourceFile),
        );
      }
    }

    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken) {
      const sink = taintSinkForAssignment(node.left);
      if (sink && expressionTaint(node.right, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.left.getStart(sourceFile),
        );
      }
    }

    ts.forEachChild(node, visitTaintNode);
  }
}

function callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache) {
  const args = Array.isArray(node.arguments) ? node.arguments : [];
  for (const index of sink.argument_indexes || []) {
    if (index < 0 || index >= args.length) {
      continue;
    }
    if (expressionTaint(args[index], checker, symbolTaintCache, expressionTaintCache)) {
      return true;
    }
  }
  return false;
}

function expressionTaint(node, checker, symbolTaintCache, expressionTaintCache) {
  if (!node) {
    return null;
  }
  if (expressionTaintCache.has(node)) {
    return expressionTaintCache.get(node);
  }
  expressionTaintCache.set(node, null);
  const taint = computeExpressionTaint(node, checker, symbolTaintCache, expressionTaintCache);
  expressionTaintCache.set(node, taint);
  return taint;
}

function computeExpressionTaint(node, checker, symbolTaintCache, expressionTaintCache) {
  const source = directTaintSource(node, checker, bindingsForNode(node));
  if (source) {
    return source;
  }

  if (ts.isIdentifier(node)) {
    return symbolTaint(symbolForNode(node, checker), checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isCallExpression(node)) {
    return directTaintSource(node, checker, bindingsForNode(node));
  }

  if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isTypeAssertionExpression(node) || ts.isNonNullExpression(node) || ts.isAwaitExpression(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isConditionalExpression(node)) {
    return expressionTaint(node.whenTrue, checker, symbolTaintCache, expressionTaintCache) ||
      expressionTaint(node.whenFalse, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isBinaryExpression(node)) {
    return expressionTaint(node.left, checker, symbolTaintCache, expressionTaintCache) ||
      expressionTaint(node.right, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isTemplateExpression(node)) {
    for (const span of node.templateSpans) {
      const taint = expressionTaint(span.expression, checker, symbolTaintCache, expressionTaintCache);
      if (taint) {
        return taint;
      }
    }
    return null;
  }

  if (ts.isTemplateSpan(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isArrayLiteralExpression(node)) {
    for (const element of node.elements) {
      const taint = expressionTaint(element, checker, symbolTaintCache, expressionTaintCache);
      if (taint) {
        return taint;
      }
    }
    return null;
  }

  if (ts.isObjectLiteralExpression(node)) {
    for (const property of node.properties) {
      if (ts.isPropertyAssignment(property) || ts.isShorthandPropertyAssignment(property)) {
        const value = ts.isShorthandPropertyAssignment(property) ? property.name : property.initializer;
        const taint = expressionTaint(value, checker, symbolTaintCache, expressionTaintCache);
        if (taint) {
          return taint;
        }
      }
    }
  }

  return null;
}

function symbolTaint(symbol, checker, symbolTaintCache, expressionTaintCache) {
  if (!symbol) {
    return null;
  }
  if (symbolTaintCache.has(symbol)) {
    return symbolTaintCache.get(symbol);
  }
  symbolTaintCache.set(symbol, null);
  const aliased = checker.getAliasedSymbol && symbol.flags & ts.SymbolFlags.Alias ? checker.getAliasedSymbol(symbol) : symbol;
  if (aliased !== symbol) {
    const taint = symbolTaint(aliased, checker, symbolTaintCache, expressionTaintCache);
    symbolTaintCache.set(symbol, taint);
    return taint;
  }
  const declarations = symbol.declarations || [];
  for (const declaration of declarations) {
    const taint = taintFromDeclaration(declaration, checker, symbolTaintCache, expressionTaintCache);
    if (taint) {
      symbolTaintCache.set(symbol, taint);
      return taint;
    }
  }
  return null;
}

function taintFromDeclaration(declaration, checker, symbolTaintCache, expressionTaintCache) {
  if (ts.isVariableDeclaration(declaration)) {
    if (ts.isIdentifier(declaration.name)) {
      return expressionTaint(declaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    }
    if (ts.isObjectBindingPattern(declaration.name)) {
      return expressionTaint(declaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    }
  }

  if (ts.isBindingElement(declaration)) {
    const pattern = declaration.parent;
    const variableDeclaration = pattern && pattern.parent && ts.isVariableDeclaration(pattern.parent) ? pattern.parent : null;
    if (!variableDeclaration) {
      return null;
    }
    const bindingSource = expressionTaint(variableDeclaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    if (bindingSource) {
      return bindingSource;
    }
  }

  if (ts.isParameter(declaration)) {
    return directTaintSource(declaration.name, checker, bindingsForNode(declaration));
  }

  return null;
}

function directTaintSource(node, checker, bindings) {
  for (const source of taintModel.sources) {
    if (source.kind === "member-access" && isConfiguredMemberSource(node, source, checker)) {
      return source;
    }
    if (source.kind === "call-result" && isConfiguredCallSource(node, source, checker, bindings)) {
      return source;
    }
  }
  return null;
}

function isConfiguredMemberSource(node, source, checker) {
  if (!ts.isPropertyAccessExpression(node) && !ts.isElementAccessExpression(node)) {
    return false;
  }
  const property = accessedPropertyName(node);
  if (!property || !(source.base_property_names || []).includes(property)) {
    return false;
  }
  const base = node.expression;
  if (ts.isIdentifier(base) && (source.base_identifiers || []).includes(base.text)) {
    return true;
  }
  return expressionTypeMatches(base, checker, source.receiver_type_names || source.base_type_names || []);
}

function isConfiguredCallSource(node, source, checker, bindings) {
  if (!ts.isCallExpression(node) || !ts.isPropertyAccessExpression(node.expression)) {
    return false;
  }
  if (!(source.call_members || []).includes(node.expression.name.text)) {
    return false;
  }
  const receiver = node.expression.expression;
  const target = callTarget(node.expression, bindings || emptyBindings());
  return expressionTypeMatches(receiver, checker, source.receiver_type_names || []) ||
    (!!target && target.module === source.module);
}

function expressionTypeMatches(node, checker, expectedNames) {
  if (!node || !checker || !Array.isArray(expectedNames) || expectedNames.length === 0) {
    return false;
  }
  try {
    const type = checker.getTypeAtLocation(node);
    const text = checker.typeToString(type);
    return expectedNames.some((name) => text === name || text.endsWith(`.${name}`) || text.includes(name));
  } catch (error) {
    return false;
  }
}

function taintSinkForCall(node, bindings, checker) {
  for (const sink of taintModel.sinks) {
    if (sink.kind !== "call") {
      continue;
    }
    if (matchesCallSink(node, sink, bindings, checker)) {
      return sink;
    }
  }
  return null;
}

function taintSinkForNewExpression(node) {
  for (const sink of taintModel.sinks) {
    if (sink.kind === "new" && ts.isIdentifier(node.expression) && node.expression.text === sink.member) {
      return sink;
    }
  }
  return null;
}

function taintSinkForAssignment(left) {
  for (const sink of taintModel.sinks) {
    if (sink.kind === "assignment" && propertyAccessMatches(left, sink.property_name)) {
      return sink;
    }
  }
  return null;
}

function matchesCallSink(node, sink, bindings, checker) {
  if (sink.member === "document.write" || sink.member === "document.writeln") {
    return isDocumentWriteMember(node.expression, sink.member);
  }
  if (sink.module) {
    const target = callTarget(node.expression, bindings);
    return !!target && target.module === sink.module && target.member === sink.member;
  }
  if (sink.member === "eval" || sink.member === "Function") {
    return ts.isIdentifier(node.expression) && node.expression.text === sink.member;
  }
  return propertyAccessMatches(node.expression, sink.member);
}

function isDocumentWriteMember(expression, name) {
  return ts.isPropertyAccessExpression(expression) &&
    `${expression.expression.getText()}.${expression.name.text}` === name;
}

function propertyAccessMatches(node, propertyName) {
  return ts.isPropertyAccessExpression(node) && node.name.text === propertyName;
}

function accessedPropertyName(node) {
  if (ts.isPropertyAccessExpression(node)) {
    return node.name.text;
  }
  if (ts.isElementAccessExpression(node) && isStringLiteralArgument(node.argumentExpression)) {
    return literalText(node.argumentExpression);
  }
  return "";
}

function symbolForNode(node, checker) {
  if (!node || !checker) {
    return null;
  }
  try {
    return checker.getSymbolAtLocation(node);
  } catch (error) {
    return null;
  }
}

function bindingsForNode(node) {
  let current = node;
  while (current) {
    if (current.bindings) {
      return current.bindings;
    }
    current = current.parent;
  }
  return emptyBindings();
}

function emptyBindings() {
  return { named: new Map(), namespaces: new Map() };
}
