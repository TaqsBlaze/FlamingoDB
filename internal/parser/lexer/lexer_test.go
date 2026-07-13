package lexer

import (
	"testing"
)

func TestNextToken(t *testing.T) {
	input := `
		SELECT id, name FROM users WHERE age >= 18;
		CREATE TABLE items (id INT, price FLOAT);
		INSERT INTO items VALUES (1, 9.99);
		UPDATE items SET price = 10.00 WHERE id = 1;
		DELETE FROM items WHERE id != 1;
		'hello world'
		"string"
		123
		12.34
	`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{SELECT, "SELECT"},
		{IDENT, "id"},
		{COMMA, ","},
		{IDENT, "name"},
		{FROM, "FROM"},
		{IDENT, "users"},
		{WHERE, "WHERE"},
		{IDENT, "age"},
		{GTE, ">="},
		{INT, "18"},
		{SEMICOLON, ";"},

		{CREATE, "CREATE"},
		{TABLE, "TABLE"},
		{IDENT, "items"},
		{LPAREN, "("},
		{IDENT, "id"},
		{IDENT, "INT"},
		{COMMA, ","},
		{IDENT, "price"},
		{IDENT, "FLOAT"},
		{RPAREN, ")"},
		{SEMICOLON, ";"},

		{INSERT, "INSERT"},
		{INTO, "INTO"},
		{IDENT, "items"},
		{VALUES, "VALUES"},
		{LPAREN, "("},
		{INT, "1"},
		{COMMA, ","},
		{FLOAT, "9.99"},
		{RPAREN, ")"},
		{SEMICOLON, ";"},

		{UPDATE, "UPDATE"},
		{IDENT, "items"},
		{SET, "SET"},
		{IDENT, "price"},
		{ASSIGN, "="},
		{FLOAT, "10.00"},
		{WHERE, "WHERE"},
		{IDENT, "id"},
		{ASSIGN, "="},
		{INT, "1"},
		{SEMICOLON, ";"},

		{DELETE, "DELETE"},
		{FROM, "FROM"},
		{IDENT, "items"},
		{WHERE, "WHERE"},
		{IDENT, "id"},
		{NOT_EQ, "!="},
		{INT, "1"},
		{SEMICOLON, ";"},

		{STRING, "hello world"},
		{STRING, "string"},
		{INT, "123"},
		{FLOAT, "12.34"},
		{EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q (literal: %s)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
