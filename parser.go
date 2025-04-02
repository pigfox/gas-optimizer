package main

import (
	"fmt"
	"strings"
)

// TokenType defines types of tokens in Solidity
type TokenType int

const (
	TokenIdentifier TokenType = iota
	TokenKeyword
	TokenOperator
	TokenPunctuation
	TokenNumber
	TokenWhitespace
)

// Token represents a single token in the Solidity code
type Token struct {
	Type  TokenType
	Value string
	Line  int
}

// Node represents a node in the simplified AST
type Node struct {
	Type     string
	Value    string
	Children []*Node
	Line     int
}

// Parser holds the state of the parsing process
type Parser struct {
	Tokens  []Token
	Pos     int
	Source  string
	Current Token
}

// NewParser creates a new parser instance
func NewParser(source string) *Parser {
	tokens := tokenize(source)
	return &Parser{
		Tokens: tokens,
		Pos:    0,
		Source: source,
	}
}

// tokenize breaks the source code into tokens
func tokenize(source string) []Token {
	var tokens []Token
	lines := strings.Split(source, "\n")
	keywords := map[string]bool{
		"for": true, "while": true, "if": true, "function": true,
		"uint": true, "public": true, "mapping": true, "returns": true,
	}
	operators := map[string]bool{"=": true, ".": true, ";": true, "<": true, "++": true}
	punctuation := map[string]bool{"(": true, ")": true, "{": true, "}": true}

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var current string
		for i := 0; i < len(line); i++ {
			char := string(line[i])

			if char == " " || char == "\t" {
				if current != "" {
					tokens = append(tokens, classifyToken(current, lineNum+1, keywords))
					current = ""
				}
				continue
			}

			if operators[char] || punctuation[char] {
				if current != "" {
					tokens = append(tokens, classifyToken(current, lineNum+1, keywords))
					current = ""
				}
				tokType := TokenOperator
				if punctuation[char] {
					tokType = TokenPunctuation
				}
				tokens = append(tokens, Token{Type: tokType, Value: char, Line: lineNum + 1})
				continue
			}

			current += char
		}
		if current != "" {
			tokens = append(tokens, classifyToken(current, lineNum+1, keywords))
		}
	}
	return tokens
}

// classifyToken determines the type of a token
func classifyToken(value string, line int, keywords map[string]bool) Token {
	if keywords[value] {
		return Token{Type: TokenKeyword, Value: value, Line: line}
	}
	if _, err := fmt.Sscanf(value, "%d", new(int)); err == nil {
		return Token{Type: TokenNumber, Value: value, Line: line}
	}
	return Token{Type: TokenIdentifier, Value: value, Line: line}
}

// Parse constructs a simplified AST
func (p *Parser) Parse() *Node {
	root := &Node{Type: "Root", Children: []*Node{}}
	p.advance()

	for p.Pos < len(p.Tokens) {
		switch p.Current.Type {
		case TokenKeyword:
			switch p.Current.Value {
			case "for":
				if forNode := p.parseForLoop(); forNode != nil {
					root.Children = append(root.Children, forNode)
				}
			case "while":
				if whileNode := p.parseWhileLoop(); whileNode != nil {
					root.Children = append(root.Children, whileNode)
				}
			case "if":
				if ifNode := p.parseIfStatement(); ifNode != nil {
					root.Children = append(root.Children, ifNode)
				}
			case "function":
				if funcNode := p.parseFunction(); funcNode != nil {
					root.Children = append(root.Children, funcNode)
				}
			default:
				p.advance()
			}
		default:
			p.advance()
		}
	}
	return root
}

// advance moves to the next token
func (p *Parser) advance() {
	if p.Pos < len(p.Tokens) {
		p.Current = p.Tokens[p.Pos]
		p.Pos++
	}
}

// parseForLoop parses a for loop structure
func (p *Parser) parseForLoop() *Node {
	forNode := &Node{Type: "ForStatement", Line: p.Current.Line}
	p.advance() // Skip 'for'
	return p.parseLoop(forNode)
}

// parseWhileLoop parses a while loop structure
func (p *Parser) parseWhileLoop() *Node {
	whileNode := &Node{Type: "WhileStatement", Line: p.Current.Line}
	p.advance() // Skip 'while'
	return p.parseLoop(whileNode)
}

// parseLoop is a helper for parsing loop bodies
func (p *Parser) parseLoop(node *Node) *Node {
	if p.Current.Type != TokenPunctuation || p.Current.Value != "(" {
		return nil
	}
	p.advance()

	for p.Current.Value != ")" && p.Pos < len(p.Tokens) {
		p.advance()
	}
	p.advance() // Skip ')'

	if p.Current.Type == TokenPunctuation && p.Current.Value == "{" {
		p.advance()
		body := &Node{Type: "Block", Line: p.Current.Line}
		for p.Current.Value != "}" && p.Pos < len(p.Tokens) {
			if p.Current.Type == TokenIdentifier {
				if access := p.parseVariableAccess(); access != nil {
					body.Children = append(body.Children, access)
				}
			}
			p.advance()
		}
		node.Children = append(node.Children, body)
		p.advance() // Skip '}'
	}
	return node
}

// parseIfStatement parses an if statement
func (p *Parser) parseIfStatement() *Node {
	ifNode := &Node{Type: "IfStatement", Line: p.Current.Line}
	p.advance() // Skip 'if'

	if p.Current.Type != TokenPunctuation || p.Current.Value != "(" {
		return nil
	}
	p.advance()

	for p.Current.Value != ")" && p.Pos < len(p.Tokens) {
		p.advance()
	}
	p.advance() // Skip ')'

	if p.Current.Type == TokenPunctuation && p.Current.Value == "{" {
		p.advance()
		body := &Node{Type: "Block", Line: p.Current.Line}
		for p.Current.Value != "}" && p.Pos < len(p.Tokens) {
			if p.Current.Type == TokenIdentifier {
				if access := p.parseVariableAccess(); access != nil {
					body.Children = append(body.Children, access)
				}
			}
			p.advance()
		}
		ifNode.Children = append(ifNode.Children, body)
		p.advance() // Skip '}'
	}
	return ifNode
}

// parseFunction parses a function declaration
func (p *Parser) parseFunction() *Node {
	funcNode := &Node{Type: "FunctionDeclaration", Line: p.Current.Line}
	p.advance() // Skip 'function'

	if p.Current.Type == TokenIdentifier {
		funcNode.Value = p.Current.Value // Function name
		p.advance()
	}

	if p.Current.Type == TokenPunctuation && p.Current.Value == "(" {
		p.advance()
		for p.Current.Value != ")" && p.Pos < len(p.Tokens) {
			p.advance()
		}
		p.advance() // Skip ')'
	}

	if p.Current.Type == TokenPunctuation && p.Current.Value == "{" {
		p.advance()
		body := &Node{Type: "Block", Line: p.Current.Line}
		for p.Current.Value != "}" && p.Pos < len(p.Tokens) {
			if p.Current.Type == TokenKeyword {
				switch p.Current.Value {
				case "for":
					if forNode := p.parseForLoop(); forNode != nil {
						body.Children = append(body.Children, forNode)
					}
				case "while":
					if whileNode := p.parseWhileLoop(); whileNode != nil {
						body.Children = append(body.Children, whileNode)
					}
				case "if":
					if ifNode := p.parseIfStatement(); ifNode != nil {
						body.Children = append(body.Children, ifNode)
					}
				}
			} else if p.Current.Type == TokenIdentifier {
				if access := p.parseVariableAccess(); access != nil {
					body.Children = append(body.Children, access)
				}
			}
			p.advance()
		}
		funcNode.Children = append(funcNode.Children, body)
		p.advance() // Skip '}'
	}
	return funcNode
}

// parseVariableAccess parses a variable access (e.g., data[i])
func (p *Parser) parseVariableAccess() *Node {
	node := &Node{Type: "MemberAccess", Value: p.Current.Value, Line: p.Current.Line}
	p.advance()

	if p.Current.Type == TokenOperator && p.Current.Value == "." {
		p.advance()
		if p.Current.Type == TokenIdentifier {
			node.Children = append(node.Children, &Node{Type: "Identifier", Value: p.Current.Value, Line: p.Current.Line})
			p.advance()
		}
	}
	return node
}
