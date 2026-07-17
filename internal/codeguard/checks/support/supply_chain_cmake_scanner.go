package support

import (
	"unicode"
)

type cmakeArgument struct {
	value string
	line  int
}

type cmakeCommand struct {
	name string
	args []cmakeArgument
	line int
}

type cmakeScanPosition struct {
	offset int
	line   int
}

func scanCMakeCommands(source string) []cmakeCommand {
	commands := make([]cmakeCommand, 0)
	position := cmakeScanPosition{line: 1}
	for position.offset < len(source) {
		command, next, ok := scanCMakeCommandAt(source, position)
		position = next
		if ok {
			commands = append(commands, command)
		}
	}
	return commands
}

func scanCMakeCommandAt(source string, position cmakeScanPosition) (cmakeCommand, cmakeScanPosition, bool) {
	position = skipCMakeTrivia(source, position)
	if position.offset >= len(source) {
		return cmakeCommand{}, position, false
	}
	if !isCMakeIdentifierStart(source[position.offset]) {
		position.offset++
		return cmakeCommand{}, position, false
	}
	start, commandLine := position.offset, position.line
	for position.offset < len(source) && isCMakeIdentifierPart(source[position.offset]) {
		position.offset++
	}
	name := source[start:position.offset]
	position = skipCMakeWhitespace(source, position)
	if position.offset >= len(source) || source[position.offset] != '(' {
		return cmakeCommand{}, position, false
	}
	position.offset++
	args, next, ok := scanCMakeArguments(source, position)
	return cmakeCommand{name: name, args: args, line: commandLine}, next, ok
}

func scanCMakeArguments(source string, position cmakeScanPosition) ([]cmakeArgument, cmakeScanPosition, bool) {
	args := make([]cmakeArgument, 0)
	depth := 1
	for position.offset < len(source) {
		position = skipCMakeTrivia(source, position)
		if position.offset >= len(source) {
			break
		}
		if source[position.offset] == ')' {
			depth--
			position.offset++
			if depth == 0 {
				return args, position, true
			}
			continue
		}
		arg, next, nestedDepth, ok := scanCMakeArgumentAt(source, position)
		if !ok {
			return args, next, false
		}
		args = append(args, arg)
		depth += nestedDepth
		position = next
	}
	return args, position, false
}

func scanCMakeArgumentAt(source string, position cmakeScanPosition) (cmakeArgument, cmakeScanPosition, int, bool) {
	line := position.line
	switch source[position.offset] {
	case '"':
		value, next, ok := scanCMakeQuoted(source, position)
		return cmakeArgument{value: value, line: line}, next, 0, ok
	case '[':
		value, next, ok := scanCMakeBracketArgument(source, position)
		if ok {
			return cmakeArgument{value: value, line: line}, next, 0, true
		}
	}
	value, next, nestedDepth := scanCMakeUnquoted(source, position)
	return cmakeArgument{value: value, line: line}, next, nestedDepth, value != ""
}

func scanCMakeUnquoted(source string, position cmakeScanPosition) (string, cmakeScanPosition, int) {
	start, nestedDepth := position.offset, 0
	for position.offset < len(source) && !unicode.IsSpace(rune(source[position.offset])) && source[position.offset] != ')' && source[position.offset] != '#' {
		if source[position.offset] == '(' {
			nestedDepth++
		}
		position.offset++
	}
	return source[start:position.offset], position, nestedDepth
}
