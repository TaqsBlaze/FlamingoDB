package lexer

// TokenType represents a lexical token type.
type TokenType string

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Identifiers and literals
	IDENT  = "IDENT"
	INT    = "INT"
	FLOAT  = "FLOAT"
	STRING = "STRING"

	// Operators
	ASSIGN   = "="
	PLUS     = "+"
	MINUS    = "-"
	BANG     = "!"
	ASTERISK = "*"
	SLASH    = "/"
	LT       = "<"
	GT       = ">"
	LTE      = "<="
	GTE      = ">="
	EQ       = "=="
	NOT_EQ   = "!="

	// Delimiters
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACKET  = "["
	RBRACKET  = "]"

// Join condition
	ON        = "ON"

	// Imaginary suffix
	IMAGINARY = "IMAGINARY"

	// Keywords
	CREATE   = "CREATE"
	TABLE    = "TABLE"
	SELECT   = "SELECT"
	INSERT   = "INSERT"
	UPDATE   = "UPDATE"
	DELETE   = "DELETE"
	WHERE    = "WHERE"
	VALUES   = "VALUES"
	FROM     = "FROM"
	INTO     = "INTO"
	SET      = "SET"
	DROP     = "DROP"
	SHOW     = "SHOW"
	TABLES   = "TABLES"

	// Aggregate functions
	COUNT    = "COUNT"
	SUM      = "SUM"
	AVG      = "AVG"

	// JOIN types
	INNER    = "INNER"
	LEFT     = "LEFT"
	JOIN     = "JOIN"

	// Clauses
	DISTINCT = "DISTINCT"
	GROUP    = "GROUP"
	BY       = "BY"
	ORDER    = "ORDER"
	HAVING   = "HAVING"
	LIMIT    = "LIMIT"
	OFFSET   = "OFFSET"
	IF       = "IF"
	EXISTS   = "EXISTS"

	// Constraints
	AUTO_INCREMENT = "AUTO_INCREMENT"
	PRIMARY        = "PRIMARY"
	KEY            = "KEY"
	NOT            = "NOT"
	NULL           = "NULL"
	UNIQUE         = "UNIQUE"

	// Index operations
	INDEX     = "INDEX"
	INDEXES   = "INDEXES"

// Order modifiers
	ASC      = "ASC"
	DESC     = "DESC"
)

// Token represents a lexical token.
type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]TokenType{
	"CREATE":    CREATE,
	"TABLE":     TABLE,
	"SELECT":    SELECT,
	"INSERT":    INSERT,
	"UPDATE":    UPDATE,
	"DELETE":    DELETE,
	"WHERE":     WHERE,
	"VALUES":    VALUES,
	"FROM":      FROM,
	"INTO":      INTO,
	"SET":       SET,
	"DROP":      DROP,
	"SHOW":      SHOW,
	"TABLES":    TABLES,

	// Aggregate functions
	"COUNT": COUNT,
	"SUM":   SUM,
	"AVG":   AVG,

	// JOIN types
	"INNER": INNER,
	"LEFT":  LEFT,
	"JOIN":  JOIN,

	// Clauses
	"DISTINCT": DISTINCT,
	"GROUP":    GROUP,
	"BY":       BY,
	"ORDER":    ORDER,
	"HAVING":   HAVING,
	"LIMIT":    LIMIT,
	"OFFSET":   OFFSET,
	"IF":       IF,
	"EXISTS":   EXISTS,

	// Constraints
	"AUTO_INCREMENT": AUTO_INCREMENT,
	"PRIMARY":        PRIMARY,
	"KEY":            KEY,
	"NOT":            NOT,
	"NULL":           NULL,

	// Index operations
	"INDEX":  INDEX,
	"INDEXES": INDEXES,

// Order modifiers
	"ASC":  ASC,
	"DESC": DESC,

// Join condition
	"ON":     ON,
}

// LookupIdent checks whether the given identifier is a keyword.
func LookupIdent(ident string) TokenType {
	// Let's make keyword lookup case-insensitive
	// actually for a parser we might uppercase it before lookup or here.
	// For simplicity, we assume keywords are defined in uppercase and we'll check it in uppercase.
	// We will handle case-insensitivity in the lexer.
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
