function analyzeSecurityNode(node, sourceFile, relPath, flavor, bindings, checker) {
  if (isRejectUnauthorizedFalse(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.insecure-tls", "security.javascript.insecure-tls"),
      "fail",
      `${scriptLabel(flavor)} TLS verification is disabled`,
      node.getStart(sourceFile),
    );
  }

  if (isNodeTLSDisableAssignment(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.insecure-tls", "security.javascript.insecure-tls"),
      "fail",
      "NODE_TLS_REJECT_UNAUTHORIZED disables TLS verification",
      node.getStart(sourceFile),
    );
  }

  if (ts.isCallExpression(node)) {
    analyzeCallExpression(node, sourceFile, relPath, flavor, bindings, checker);
  }

  if (ts.isNewExpression(node) && isDynamicFunctionConstructor(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.dynamic-code", "security.javascript.dynamic-code"),
      "warn",
      "dynamic code execution should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && isUnsafeHTMLAssignment(node.left)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.unsafe-html-sink", "security.javascript.unsafe-html-sink"),
      "warn",
      "unsafe HTML injection sink should be reviewed",
      node.left.getStart(sourceFile),
    );
  }
}

function analyzeCallExpression(node, sourceFile, relPath, flavor, bindings, checker) {
  const directCallee = callTarget(node.expression, bindings);
  const rule = scriptRuleId;

  if (isEvalCall(node.expression) || isFunctionCall(node.expression)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.dynamic-code", "security.javascript.dynamic-code"), "warn", "dynamic code execution should be reviewed", node.expression.getStart(sourceFile));
  }

  if (directCallee && directCallee.module === "child_process") {
    if (directCallee.member === "exec" || directCallee.member === "execSync") {
      pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.shell-execution", "security.javascript.shell-execution"), "warn", `${scriptLabel(flavor)} shell execution primitive should be reviewed`, node.expression.getStart(sourceFile));
    }
    if ((directCallee.member === "spawn" || directCallee.member === "spawnSync") && hasShellTrueOption(node.arguments)) {
      pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.shell-execution", "security.javascript.shell-execution"), "warn", `${scriptLabel(flavor)} shell execution primitive should be reviewed`, node.expression.getStart(sourceFile));
    }
  }

  if (directCallee && directCallee.module === "vm" && isVMExecutionMember(directCallee.member)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.vm-dynamic-code", "security.javascript.vm-dynamic-code"), "warn", "Node vm dynamic code execution should be reviewed", node.expression.getStart(sourceFile));
  }

  if (isInsertAdjacentHTML(node.expression) || isDocumentWrite(node.expression)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.unsafe-html-sink", "security.javascript.unsafe-html-sink"), "warn", "unsafe HTML injection sink should be reviewed", node.expression.getStart(sourceFile));
  }

  if (isTimerCall(node.expression) && node.arguments.length > 0 && isStringLiteralArgument(node.arguments[0])) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.string-timer-code", "security.javascript.string-timer-code"), "warn", `${scriptLabel(flavor)} string-based timer execution should be reviewed`, node.expression.getStart(sourceFile));
  }

  if (isPostMessageCall(node.expression) && node.arguments.length > 1 && isWildcardString(node.arguments[1])) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.postmessage-wildcard", "security.javascript.postmessage-wildcard"), "warn", `${scriptLabel(flavor)} postMessage wildcard origin should be reviewed`, node.expression.getStart(sourceFile));
  }
}

function isRejectUnauthorizedFalse(node) {
  return ts.isPropertyAssignment(node) &&
    propertyName(node.name) === "rejectUnauthorized" &&
    node.initializer.kind === ts.SyntaxKind.FalseKeyword;
}

function isNodeTLSDisableAssignment(node) {
  return ts.isBinaryExpression(node) &&
    node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
    isNodeTLSIdentifier(node.left) &&
    isZeroLiteral(node.right);
}

function isNodeTLSIdentifier(node) {
  if (ts.isIdentifier(node)) {
    return node.text === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  if (ts.isPropertyAccessExpression(node)) {
    return node.name.text === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  if (ts.isElementAccessExpression(node) && isStringLiteralArgument(node.argumentExpression)) {
    return literalText(node.argumentExpression) === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  return false;
}

function isZeroLiteral(node) {
  return (ts.isStringLiteralLike(node) && node.text === "0") ||
    (ts.isNumericLiteral(node) && node.text === "0");
}

function collectBindings(sourceFile) {
  const named = new Map();
  const namespaces = new Map();

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement) && ts.isStringLiteral(statement.moduleSpecifier)) {
      const moduleName = stripNodePrefix(statement.moduleSpecifier.text);
      const clause = statement.importClause;
      if (!clause) {
        continue;
      }
      if (clause.name) {
        namespaces.set(clause.name.text, moduleName);
      }
      if (clause.namedBindings) {
        if (ts.isNamespaceImport(clause.namedBindings)) {
          namespaces.set(clause.namedBindings.name.text, moduleName);
        } else if (ts.isNamedImports(clause.namedBindings)) {
          for (const element of clause.namedBindings.elements) {
            named.set(element.name.text, {
              module: moduleName,
              member: element.propertyName ? element.propertyName.text : element.name.text,
            });
          }
        }
      }
      continue;
    }

    if (!ts.isVariableStatement(statement)) {
      continue;
    }
    for (const declaration of statement.declarationList.declarations) {
      collectRequireBinding(declaration, named, namespaces);
    }
  }

  return { named, namespaces };
}

function collectRequireBinding(declaration, named, namespaces) {
  const directModule = requireModuleName(declaration.initializer);
  if (directModule) {
    if (ts.isIdentifier(declaration.name)) {
      namespaces.set(declaration.name.text, directModule);
    }
    if (ts.isObjectBindingPattern(declaration.name)) {
      for (const element of declaration.name.elements) {
        const alias = element.name && ts.isIdentifier(element.name) ? element.name.text : "";
        if (!alias) {
          continue;
        }
        named.set(alias, {
          module: directModule,
          member: element.propertyName && ts.isIdentifier(element.propertyName) ? element.propertyName.text : alias,
        });
      }
    }
    return;
  }

  if (!ts.isIdentifier(declaration.name) || !ts.isPropertyAccessExpression(declaration.initializer || {})) {
    return;
  }
  const moduleName = requireModuleName(declaration.initializer.expression);
  if (!moduleName) {
    return;
  }
  named.set(declaration.name.text, {
    module: moduleName,
    member: declaration.initializer.name.text,
  });
}

function requireModuleName(node) {
  if (!node || !ts.isCallExpression(node) || !ts.isIdentifier(node.expression) || node.expression.text !== "require" || node.arguments.length !== 1) {
    return "";
  }
  if (!ts.isStringLiteral(node.arguments[0])) {
    return "";
  }
  return stripNodePrefix(node.arguments[0].text);
}

function stripNodePrefix(moduleName) {
  return String(moduleName || "").replace(/^node:/, "");
}

function callTarget(expression, bindings) {
  if (ts.isIdentifier(expression) && bindings.named.has(expression.text)) {
    return bindings.named.get(expression.text);
  }
  if (ts.isPropertyAccessExpression(expression)) {
    if (ts.isIdentifier(expression.expression) && bindings.namespaces.has(expression.expression.text)) {
      return { module: bindings.namespaces.get(expression.expression.text), member: expression.name.text };
    }
    const moduleName = requireModuleName(expression.expression);
    if (moduleName) {
      return { module: moduleName, member: expression.name.text };
    }
  }
  return null;
}

function hasShellTrueOption(args) {
  return args.some((argument) =>
    ts.isObjectLiteralExpression(argument) &&
    argument.properties.some((property) =>
      ts.isPropertyAssignment(property) &&
      propertyName(property.name) === "shell" &&
      property.initializer.kind === ts.SyntaxKind.TrueKeyword,
    ),
  );
}

function propertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  if (ts.isComputedPropertyName(name) && ts.isStringLiteralLike(name.expression)) {
    return name.expression.text;
  }
  return "";
}

function isVMExecutionMember(member) {
  return member === "runInContext" ||
    member === "runInNewContext" ||
    member === "runInThisContext" ||
    member === "compileFunction";
}

function isEvalCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "eval";
}

function isFunctionCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "Function";
}

function isDynamicFunctionConstructor(node) {
  return ts.isIdentifier(node.expression) && node.expression.text === "Function";
}

function isUnsafeHTMLAssignment(node) {
  return ts.isPropertyAccessExpression(node) &&
    (node.name.text === "innerHTML" || node.name.text === "outerHTML");
}

function isInsertAdjacentHTML(expression) {
  return ts.isPropertyAccessExpression(expression) && expression.name.text === "insertAdjacentHTML";
}

function isDocumentWrite(expression) {
  return ts.isPropertyAccessExpression(expression) &&
    (expression.name.text === "write" || expression.name.text === "writeln") &&
    ts.isIdentifier(expression.expression) &&
    expression.expression.text === "document";
}

function isTimerCall(expression) {
  return ts.isIdentifier(expression) &&
    (expression.text === "setTimeout" || expression.text === "setInterval");
}

function isPostMessageCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "postMessage" ||
    ts.isPropertyAccessExpression(expression) && expression.name.text === "postMessage";
}

function isStringLiteralArgument(node) {
  return ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node);
}

function isWildcardString(node) {
  return isStringLiteralArgument(node) && literalText(node) === "*";
}

function literalText(node) {
  return node.text || "";
}
