package ast

import (
	"github.com/TaqsBlaze/FlamingoDB/internal/parser/lexer"
	"strings"
)

// Node is the base interface for all AST nodes.
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents a statement node in the AST.
type Statement interface {
	Node
	statementNode()
}

// Expression represents an expression node in the AST.
type Expression interface {
	Node
	expressionNode()
}

// Program is the root node of the AST.
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out string
	for _, s := range p.Statements {
		out += s.String()
	}
	return out
}

// Identifier represents a column name or table name.
type Identifier struct {
	Token lexer.Token // the IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// IntegerLiteral represents an integer.
type IntegerLiteral struct {
	Token lexer.Token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// FloatLiteral represents a float.
type FloatLiteral struct {
	Token lexer.Token
	Value float64
}

func (fl *FloatLiteral) expressionNode()      {}
func (fl *FloatLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FloatLiteral) String() string       { return fl.Token.Literal }

// StringLiteral represents a string.
type StringLiteral struct {
	Token lexer.Token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return "'" + sl.Value + "'" }

// InfixExpression represents binary operations (e.g. id = 1, age >= 18).
type InfixExpression struct {
	Token    lexer.Token // The operator token, e.g. =
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	return "(" + ie.Left.String() + " " + ie.Operator + " " + ie.Right.String() + ")"
}

// PrefixExpression represents unary prefix operations (e.g. -x).
type PrefixExpression struct {
	Token    lexer.Token // The operator token, e.g. -
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	return "(" + pe.Operator + pe.Right.String() + ")"
}

// CallExpression represents a function call (e.g. SIN(x)).
type CallExpression struct {
	Token    lexer.Token // The IDENT token (the function name)
	Function string      // e.g. "SIN"
	Args     []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var args []string
	for _, a := range ce.Args {
		args = append(args, a.String())
	}
	return ce.Function + "(" + strings.Join(args, ", ") + ")"
}

// ColumnDef represents a column definition in a CREATE TABLE statement.
type ColumnDef struct {
	Name string
	Type string // e.g. INT, FLOAT, STRING
}

// SelectStatement represents a SELECT query.
type SelectStatement struct {
	Token lexer.Token // the 'SELECT' token
	Fields []Expression
	Table  string
	Where  Expression
}

func (s *SelectStatement) statementNode()       {}
func (s *SelectStatement) TokenLiteral() string { return s.Token.Literal }
func (s *SelectStatement) String() string {
	var fields []string
	for _, f := range s.Fields {
		fields = append(fields, f.String())
	}
	out := "SELECT " + strings.Join(fields, ", ") + " FROM " + s.Table
	if s.Where != nil {
		out += " WHERE " + s.Where.String()
	}
	return out + ";"
}

// InsertStatement represents an INSERT query.
type InsertStatement struct {
	Token  lexer.Token // the 'INSERT' token
	Table  string
	Values []Expression
}

func (s *InsertStatement) statementNode()       {}
func (s *InsertStatement) TokenLiteral() string { return s.Token.Literal }
func (s *InsertStatement) String() string {
	var vals []string
	for _, v := range s.Values {
		vals = append(vals, v.String())
	}
	return "INSERT INTO " + s.Table + " VALUES (" + strings.Join(vals, ", ") + ");"
}

// UpdateStatement represents an UPDATE query.
type UpdateStatement struct {
	Token lexer.Token // the 'UPDATE' token
	Table string
	Set   map[string]Expression
	Where Expression
}

func (s *UpdateStatement) statementNode()       {}
func (s *UpdateStatement) TokenLiteral() string { return s.Token.Literal }
func (s *UpdateStatement) String() string {
	out := "UPDATE " + s.Table + " SET "
	var sets []string
	for k, v := range s.Set {
		sets = append(sets, k+" = "+v.String())
	}
	out += strings.Join(sets, ", ")
	if s.Where != nil {
		out += " WHERE " + s.Where.String()
	}
	return out + ";"
}

// DeleteStatement represents a DELETE query.
type DeleteStatement struct {
	Token lexer.Token // the 'DELETE' token
	Table string
	Where Expression
}

func (s *DeleteStatement) statementNode()       {}
func (s *DeleteStatement) TokenLiteral() string { return s.Token.Literal }
func (s *DeleteStatement) String() string {
	out := "DELETE FROM " + s.Table
	if s.Where != nil {
		out += " WHERE " + s.Where.String()
	}
	return out + ";"
}

// CreateTableStatement represents a CREATE TABLE query.
type CreateTableStatement struct {
	Token   lexer.Token // the 'CREATE' token
	Table   string
	Columns []ColumnDef
}

func (s *CreateTableStatement) statementNode()       {}
func (s *CreateTableStatement) TokenLiteral() string { return s.Token.Literal }
func (s *CreateTableStatement) String() string {
	out := "CREATE TABLE " + s.Table + " ("
	var cols []string
	for _, c := range s.Columns {
		cols = append(cols, c.Name+" "+c.Type)
	}
	out += strings.Join(cols, ", ") + ");"
	return out
}

// DropTableStatement represents a DROP TABLE query.
type DropTableStatement struct {
	Token lexer.Token // the 'DROP' token
	Table string
}

func (s *DropTableStatement) statementNode()       {}
func (s *DropTableStatement) TokenLiteral() string { return s.Token.Literal }
func (s *DropTableStatement) String() string {
	return "DROP TABLE " + s.Table + ";"
}

// ShowTablesStatement represents a SHOW TABLES query.
type ShowTablesStatement struct {
	Token lexer.Token // the 'SHOW' token
}

func (s *ShowTablesStatement) statementNode()       {}
func (s *ShowTablesStatement) TokenLiteral() string { return s.Token.Literal }
func (s *ShowTablesStatement) String() string       { return "SHOW TABLES;" }


// ImaginaryLiteral represents an imaginary number literal (e.g. 3i, 4.5i).
type ImaginaryLiteral struct {
	Token lexer.Token
	Value float64
}

func (il *ImaginaryLiteral) expressionNode()      {}
func (il *ImaginaryLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *ImaginaryLiteral) String() string       { return il.Token.Literal }

// ArrayLiteral represents vector/matrix/tensor literal syntaxes like [1, 2, 3].
type ArrayLiteral struct {
	Token    lexer.Token // the "[" token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var elems []string
	for _, el := range al.Elements {
		elems = append(elems, el.String())
	}
	return "[" + strings.Join(elems, ", ") + "]"
}
