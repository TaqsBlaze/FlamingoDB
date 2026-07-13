package parser

import (
	"flamingodb/internal/parser/ast"
	"flamingodb/internal/parser/lexer"
	"fmt"
	"strconv"
)

// Precedences for Pratt parser
const (
	_ int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > or <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X or !X
)

var precedences = map[lexer.TokenType]int{
	lexer.EQ:       EQUALS,
	lexer.NOT_EQ:   EQUALS,
	lexer.ASSIGN:   EQUALS,
	lexer.LT:       LESSGREATER,
	lexer.GT:       LESSGREATER,
	lexer.LTE:      LESSGREATER,
	lexer.GTE:      LESSGREATER,
	lexer.PLUS:     SUM,
	lexer.MINUS:    SUM,
	lexer.SLASH:    PRODUCT,
	lexer.ASTERISK: PRODUCT,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

// Parser parses tokens into an AST.
type Parser struct {
	l *lexer.Lexer

	curToken  lexer.Token
	peekToken lexer.Token

	errors []string

	prefixParseFns map[lexer.TokenType]prefixParseFn
	infixParseFns  map[lexer.TokenType]infixParseFn
}

// New creates a new Parser.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[lexer.TokenType]prefixParseFn)
	p.registerPrefix(lexer.IDENT, p.parseIdentifier)
	p.registerPrefix(lexer.INT, p.parseIntegerLiteral)
	p.registerPrefix(lexer.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(lexer.STRING, p.parseStringLiteral)
	p.registerPrefix(lexer.MINUS, p.parsePrefixExpression)
	p.registerPrefix(lexer.IMAGINARY, p.parseImaginaryLiteral)
	p.registerPrefix(lexer.LBRACKET, p.parseArrayLiteral)

	p.infixParseFns = make(map[lexer.TokenType]infixParseFn)
	p.registerInfix(lexer.EQ, p.parseInfixExpression)
	p.registerInfix(lexer.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(lexer.ASSIGN, p.parseInfixExpression)
	p.registerInfix(lexer.LT, p.parseInfixExpression)
	p.registerInfix(lexer.GT, p.parseInfixExpression)
	p.registerInfix(lexer.LTE, p.parseInfixExpression)
	p.registerInfix(lexer.GTE, p.parseInfixExpression)
	p.registerInfix(lexer.PLUS, p.parseInfixExpression)
	p.registerInfix(lexer.MINUS, p.parseInfixExpression)
	p.registerInfix(lexer.SLASH, p.parseInfixExpression)
	p.registerInfix(lexer.ASTERISK, p.parseInfixExpression)

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t lexer.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t lexer.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type)
	p.errors = append(p.errors, msg)
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) registerPrefix(tokenType lexer.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType lexer.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// ParseProgram parses the input and returns a Program AST root node.
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case lexer.SELECT:
		return p.parseSelectStatement()
	case lexer.INSERT:
		return p.parseInsertStatement()
	case lexer.UPDATE:
		return p.parseUpdateStatement()
	case lexer.DELETE:
		return p.parseDeleteStatement()
	case lexer.CREATE:
		return p.parseCreateTableStatement()
	default:
		// Not implemented or error
		return nil
	}
}

func (p *Parser) parseSelectStatement() *ast.SelectStatement {
	stmt := &ast.SelectStatement{Token: p.curToken}

	p.nextToken() // move past SELECT

	// Parse fields
	for {
		if p.curTokenIs(lexer.IDENT) || p.curTokenIs(lexer.ASTERISK) {
			stmt.Fields = append(stmt.Fields, p.curToken.Literal)
		}
		if p.peekTokenIs(lexer.COMMA) {
			p.nextToken() // Move to COMMA
			p.nextToken() // Move past COMMA
		} else {
			break
		}
	}

	if !p.expectPeek(lexer.FROM) {
		return nil
	}
	
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken() // Move to WHERE
		p.nextToken() // Move past WHERE
		stmt.Where = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseInsertStatement() *ast.InsertStatement {
	stmt := &ast.InsertStatement{Token: p.curToken}

	if !p.expectPeek(lexer.INTO) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.VALUES) {
		return nil
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	p.nextToken() // Move past (

	for !p.curTokenIs(lexer.RPAREN) && !p.curTokenIs(lexer.EOF) {
		expr := p.parseExpression(LOWEST)
		if expr != nil {
			stmt.Values = append(stmt.Values, expr)
		}
		
		if p.peekTokenIs(lexer.COMMA) {
			p.nextToken()
			p.nextToken()
		} else {
			p.nextToken() // should be RPAREN
		}
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseUpdateStatement() *ast.UpdateStatement {
	stmt := &ast.UpdateStatement{Token: p.curToken, Set: make(map[string]ast.Expression)}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.SET) {
		return nil
	}

	p.nextToken() // Move past SET

	for {
		if !p.curTokenIs(lexer.IDENT) {
			break
		}
		col := p.curToken.Literal

		if !p.expectPeek(lexer.ASSIGN) {
			return nil
		}
		
		p.nextToken() // Move past =
		
		expr := p.parseExpression(LOWEST)
		stmt.Set[col] = expr
		
		if p.peekTokenIs(lexer.COMMA) {
			p.nextToken() // move to COMMA
			p.nextToken() // move past COMMA
		} else {
			break
		}
	}

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken()
		p.nextToken()
		stmt.Where = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseDeleteStatement() *ast.DeleteStatement {
	stmt := &ast.DeleteStatement{Token: p.curToken}

	if !p.expectPeek(lexer.FROM) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if p.peekTokenIs(lexer.WHERE) {
		p.nextToken() // Move to WHERE
		p.nextToken() // Move to condition
		stmt.Where = p.parseExpression(LOWEST)
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseCreateTableStatement() *ast.CreateTableStatement {
	stmt := &ast.CreateTableStatement{Token: p.curToken}

	if !p.expectPeek(lexer.TABLE) {
		return nil
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Table = p.curToken.Literal

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	
	p.nextToken() // move past (

	for !p.curTokenIs(lexer.RPAREN) && !p.curTokenIs(lexer.EOF) {
		if !p.curTokenIs(lexer.IDENT) {
			return nil
		}
		colName := p.curToken.Literal
		
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		colType := p.curToken.Literal
		
		stmt.Columns = append(stmt.Columns, ast.ColumnDef{Name: colName, Type: colType})
		
		if p.peekTokenIs(lexer.COMMA) {
			p.nextToken() // COMMA
			p.nextToken() // past COMMA
		} else {
			p.nextToken() // RPAREN
		}
	}

	if p.peekTokenIs(lexer.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// Expressions
func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.errors = append(p.errors, fmt.Sprintf("no prefix parse function for %s found", p.curToken.Type))
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(lexer.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// parsePrefixExpression handles unary prefix operators such as '-' for negative numbers.
func (p *Parser) parsePrefixExpression() ast.Expression {
	operatorToken := p.curToken
	p.nextToken()
	right := p.parseExpression(PREFIX)
	if right == nil {
		return nil
	}
	// For simple numeric negation, fold the sign directly into the literal value.
	switch r := right.(type) {
	case *ast.IntegerLiteral:
		r.Value = -r.Value
		r.Token = operatorToken
		return r
	case *ast.FloatLiteral:
		r.Value = -r.Value
		r.Token = operatorToken
		return r
	case *ast.ImaginaryLiteral:
		r.Value = -r.Value
		r.Token = operatorToken
		return r
	}
	return &ast.PrefixExpression{Token: operatorToken, Operator: operatorToken.Literal, Right: right}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{Token: p.curToken}
	value, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("could not parse %q as integer", p.curToken.Literal))
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{Token: p.curToken}
	value, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("could not parse %q as float", p.curToken.Literal))
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{Token: p.curToken, Value: p.curToken.Literal}
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.InfixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(precedence)

	return expression
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}


func (p *Parser) parseImaginaryLiteral() ast.Expression {
	lit := &ast.ImaginaryLiteral{Token: p.curToken}
	litStr := p.curToken.Literal
	if len(litStr) > 0 && litStr[len(litStr)-1] == 'i' {
		litStr = litStr[:len(litStr)-1]
	}
	value, err := strconv.ParseFloat(litStr, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("could not parse %q as imaginary coefficient", p.curToken.Literal))
		return nil
	}
	lit.Value = value
	return lit
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	lit := &ast.ArrayLiteral{Token: p.curToken}
	lit.Elements = []ast.Expression{}

	if p.peekTokenIs(lexer.RBRACKET) {
		p.nextToken()
		return lit
	}

	p.nextToken()
	lit.Elements = append(lit.Elements, p.parseExpression(LOWEST))

	for p.peekTokenIs(lexer.COMMA) {
		p.nextToken() // COMMA
		p.nextToken() // expression
		lit.Elements = append(lit.Elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}

	return lit
}
