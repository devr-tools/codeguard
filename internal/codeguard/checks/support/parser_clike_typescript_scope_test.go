package support

import "testing"

func TestTypeScriptParserBoundsDeclarationsToLexicalBlocks(t *testing.T) {
	parsed := ParseCLike(`export function prepareUser(repo: Repository, user: User, enabled: boolean) {
  const collaborator = repo;
  if (enabled) {
    const collaborator = new Repository();
    function loadLocal() {
      collaborator.save(user);
      return user;
    }
    loadLocal();
  }
  function loadOuter() {
    collaborator.save(user);
    return user;
  }
  return loadOuter();
}`, CLikeTypeScript)
	parent := parsed.FunctionByName("prepareUser")
	local := parsed.FunctionByName("loadLocal")
	outer := parsed.FunctionByName("loadOuter")
	if parent == nil || local == nil || outer == nil {
		t.Fatalf("functions = %#v, want parent and nested functions", parsed.AllFunctions())
	}
	var declarations []ParsedDeclaration
	for _, declaration := range parent.Declarations {
		if declaration.Name == "collaborator" {
			declarations = append(declarations, declaration)
		}
	}
	if len(declarations) != 2 {
		t.Fatalf("collaborator declarations = %#v, want outer and shadow", declarations)
	}
	outerDeclaration, innerDeclaration := declarations[0], declarations[1]
	if outerDeclaration.AliasSource != "repo" || outerDeclaration.ScopeOffsetStart >= local.DefinitionOffset || outerDeclaration.ScopeOffsetEnd <= outer.DefinitionOffset {
		t.Fatalf("outer declaration = %#v, local=%d outer=%d, want visibility at both children", outerDeclaration, local.DefinitionOffset, outer.DefinitionOffset)
	}
	if innerDeclaration.AliasSource != "" || innerDeclaration.ScopeOffsetStart >= local.DefinitionOffset || innerDeclaration.ScopeOffsetEnd <= local.DefinitionOffset {
		t.Fatalf("inner declaration = %#v, local=%d, want fresh local shadow visible to loadLocal", innerDeclaration, local.DefinitionOffset)
	}
	if innerDeclaration.ScopeOffsetStart < outer.DefinitionOffset && innerDeclaration.ScopeOffsetEnd > outer.DefinitionOffset {
		t.Fatalf("inner declaration = %#v, outer=%d, want shadow hidden from loadOuter", innerDeclaration, outer.DefinitionOffset)
	}
}
