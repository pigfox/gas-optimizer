package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
)

// Gas costs (approximate, post-EIP-2929)
const (
	GasSload = 800 // SLOAD cost
	GasMload = 3   // MLOAD cost
)

// Report represents an optimization suggestion
type Report struct {
	Issue      string
	Suggestion string
	GasSavings int
	Location   string
}

// SolcASTNode represents a node in the solc-generated AST
type SolcASTNode struct {
	NodeType         string        `json:"nodeType"`
	Name             string        `json:"name,omitempty"`
	Src              string        `json:"src"`
	Nodes            []SolcASTNode `json:"nodes,omitempty"`
	Body             *SolcASTNode  `json:"body,omitempty"`
	Statements       []SolcASTNode `json:"statements,omitempty"`
	Expression       *SolcASTNode  `json:"expression,omitempty"`
	InitialValue     *SolcASTNode  `json:"initialValue,omitempty"`
	TypeName         *SolcASTNode  `json:"typeName,omitempty"`
	TypeDescriptions *TypeDesc     `json:"typeDescriptions,omitempty"`
	Parameters       *ParamList    `json:"parameters,omitempty"`
	ReturnParameters *ParamList    `json:"returnParameters,omitempty"`
	IndexExpression  *SolcASTNode  `json:"indexExpression,omitempty"`
	BaseExpression   *SolcASTNode  `json:"baseExpression,omitempty"`
	LeftExpression   *SolcASTNode  `json:"leftExpression,omitempty"`
	RightExpression  *SolcASTNode  `json:"rightExpression,omitempty"`
	IsLValue         bool          `json:"isLValue,omitempty"`
	ReferencedDecl   int           `json:"referencedDeclaration,omitempty"`
	Operator         string        `json:"operator,omitempty"`
	Value            string        `json:"value,omitempty"`
}

type TypeDesc struct {
	TypeIdentifier string `json:"typeIdentifier"`
	TypeString     string `json:"typeString"`
}

type ParamList struct {
	Parameters []SolcASTNode `json:"parameters"`
}

// GasOptimizer holds the state of the analysis
type GasOptimizer struct {
	Source  string
	AST     interface{}
	Reports []Report
}

// NewGasOptimizer creates a new optimizer instance
func NewGasOptimizer(filePath string) (*GasOptimizer, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}
	source := string(data)

	cmd := exec.Command("solc", "--ast-compact-json", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("solc failed: %v, falling back to custom parser", err)
		parser := NewParser(source)
		ast := parser.Parse()
		return &GasOptimizer{Source: source, AST: ast, Reports: []Report{}}, nil
	}

	re := regexp.MustCompile(`(?s)JSON AST \(compact format\):.*?({.*})`)
	matches := re.FindSubmatch(output)
	if len(matches) < 2 {
		return nil, fmt.Errorf("no JSON found in solc output: %s", string(output))
	}
	jsonData := matches[1]

	var ast interface{}
	if err := json.Unmarshal(jsonData, &ast); err != nil {
		return nil, fmt.Errorf("failed to parse AST: %v, output: %s", err, string(jsonData))
	}

	return &GasOptimizer{
		Source:  source,
		AST:     ast,
		Reports: []Report{},
	}, nil
}

// Analyze runs the gas optimization analysis
func (g *GasOptimizer) Analyze() {
	switch ast := g.AST.(type) {
	case *Node:
		g.analyzeCustomAST(ast)
	case interface{}:
		g.analyzeSolcAST(ast)
	default:
		log.Println("Unknown AST type, skipping analysis")
	}
}

// analyzeCustomAST analyzes the custom parser's AST
func (g *GasOptimizer) analyzeCustomAST(ast *Node) {
	for _, node := range ast.Children {
		if node.Type == "ForStatement" || node.Type == "WhileStatement" {
			storageVars := make(map[string]int)
			g.collectStorageReadsCustom(node, storageVars)
			g.generateLoopReport(storageVars, fmt.Sprintf("line %d", node.Line))
		}
	}
}

// analyzeSolcAST analyzes the solc AST
func (g *GasOptimizer) analyzeSolcAST(ast interface{}) {
	astBytes, _ := json.Marshal(ast)
	var root SolcASTNode
	json.Unmarshal(astBytes, &root)
	g.checkLoopsForStorageReads(root)
	g.checkInefficientTypes(root)
	g.checkRedundantOperations(root)
}

// checkLoopsForStorageReads detects repeated storage reads in loops
func (g *GasOptimizer) checkLoopsForStorageReads(ast SolcASTNode) {
	g.walkSolcAST(ast, func(node SolcASTNode) {
		if node.NodeType == "ForStatement" || node.NodeType == "WhileStatement" {
			storageVars := make(map[string]int)
			if node.Body != nil {
				g.collectStorageReadsSolc(*node.Body, storageVars)
			}
			g.generateLoopReport(storageVars, node.Src)
		}
	})
}

// collectStorageReadsSolc collects storage reads from solc AST
func (g *GasOptimizer) collectStorageReadsSolc(node SolcASTNode, storageVars map[string]int) {
	if node.NodeType == "VariableDeclarationStatement" && node.InitialValue != nil {
		if iv := node.InitialValue; iv.NodeType == "IndexAccess" && iv.BaseExpression != nil && iv.IndexExpression != nil {
			varName := iv.BaseExpression.Name + "[" + iv.IndexExpression.Name + "]"
			storageVars[varName]++
		}
	}
	for _, child := range node.Statements {
		g.collectStorageReadsSolc(child, storageVars)
	}
	if node.Body != nil {
		g.collectStorageReadsSolc(*node.Body, storageVars)
	}
}

// collectStorageReadsCustom collects storage reads from custom AST
func (g *GasOptimizer) collectStorageReadsCustom(node *Node, storageVars map[string]int) {
	for _, child := range node.Children {
		if child.Type == "MemberAccess" && len(child.Children) > 0 {
			varName := child.Value + "." + child.Children[0].Value
			storageVars[varName]++
		}
		g.collectStorageReadsCustom(child, storageVars)
	}
}

// generateLoopReport creates reports for repeated storage reads
func (g *GasOptimizer) generateLoopReport(storageVars map[string]int, location string) {
	for varName, count := range storageVars {
		if count > 1 {
			savings := (count - 1) * (GasSload - GasMload)
			g.Reports = append(g.Reports, Report{
				Issue:      fmt.Sprintf("Variable '%s' read %d times in loop", varName, count),
				Suggestion: fmt.Sprintf("Cache '%s' in memory before loop", varName),
				GasSavings: savings,
				Location:   location,
			})
		}
	}
}

// checkInefficientTypes detects inefficient type usage
func (g *GasOptimizer) checkInefficientTypes(ast SolcASTNode) {
	g.walkSolcAST(ast, func(node SolcASTNode) {
		if node.NodeType == "VariableDeclaration" && node.TypeName != nil {
			typeName := node.TypeName.Name
			if typeName == "uint8" || typeName == "uint16" || typeName == "uint32" {
				g.Reports = append(g.Reports, Report{
					Issue:      fmt.Sprintf("Inefficient type '%s' used for variable '%s'", typeName, node.Name),
					Suggestion: "Use 'uint256' to avoid packing overhead unless tightly packed in a struct",
					GasSavings: 200,
					Location:   node.Src,
				})
			}
		}
	})
}

// checkRedundantOperations detects redundant computations
func (g *GasOptimizer) checkRedundantOperations(ast SolcASTNode) {
	g.walkSolcAST(ast, func(node SolcASTNode) {
		if node.NodeType == "FunctionDefinition" && node.Body != nil {
			exprMap := make(map[string]int)
			g.collectExpressions(*node.Body, exprMap)
			for expr, count := range exprMap {
				if count > 1 {
					g.Reports = append(g.Reports, Report{
						Issue:      fmt.Sprintf("Expression '%s' computed %d times", expr, count),
						Suggestion: "Cache the result in a local variable",
						GasSavings: count * 50,
						Location:   node.Src,
					})
				}
			}
		}
	})
}

// collectExpressions collects expressions for redundancy check
func (g *GasOptimizer) collectExpressions(node SolcASTNode, exprMap map[string]int) {
	if node.NodeType == "BinaryOperation" && node.LeftExpression != nil && node.RightExpression != nil {
		var leftVal, rightVal string
		if node.LeftExpression.Name != "" {
			leftVal = node.LeftExpression.Name
		} else if node.LeftExpression.Value != "" {
			leftVal = node.LeftExpression.Value
		}
		if node.RightExpression.Value != "" {
			rightVal = node.RightExpression.Value // Literal value (e.g., "2")
		} else if node.RightExpression.Name != "" {
			rightVal = node.RightExpression.Name // Identifier
		}
		if leftVal != "" && rightVal != "" {
			expr := fmt.Sprintf("%s %s %s", leftVal, node.Operator, rightVal)
			exprMap[expr]++
		}
	}
	// Recursively check nested expressions
	if node.LeftExpression != nil {
		g.collectExpressions(*node.LeftExpression, exprMap)
	}
	if node.RightExpression != nil {
		g.collectExpressions(*node.RightExpression, exprMap)
	}
	if node.NodeType == "VariableDeclarationStatement" && node.InitialValue != nil {
		g.collectExpressions(*node.InitialValue, exprMap)
	}
	if node.NodeType == "Return" && node.Expression != nil {
		g.collectExpressions(*node.Expression, exprMap)
	}
	for _, stmt := range node.Statements {
		g.collectExpressions(stmt, exprMap)
	}
}

// walkSolcAST recursively walks the solc AST
func (g *GasOptimizer) walkSolcAST(node SolcASTNode, fn func(SolcASTNode)) {
	fn(node)
	for _, child := range node.Nodes {
		g.walkSolcAST(child, fn)
	}
	if node.Body != nil {
		g.walkSolcAST(*node.Body, fn)
	}
	for _, stmt := range node.Statements {
		g.walkSolcAST(stmt, fn)
	}
}

// PrintReports displays the analysis results
func (g *GasOptimizer) PrintReports() {
	if len(g.Reports) == 0 {
		fmt.Println("No gas optimization opportunities found.")
		return
	}
	for i, r := range g.Reports {
		fmt.Printf("Report %d:\n", i+1)
		fmt.Printf("  Issue: %s\n", r.Issue)
		fmt.Printf("  Suggestion: %s\n", r.Suggestion)
		fmt.Printf("  Gas Savings: %d\n", r.GasSavings)
		fmt.Printf("  Location: %s\n\n", r.Location)
	}
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: gasoptimizer <solidity_file>")
	}

	filePath := os.Args[1]
	optimizer, err := NewGasOptimizer(filePath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	optimizer.Analyze()
	optimizer.PrintReports()
}
