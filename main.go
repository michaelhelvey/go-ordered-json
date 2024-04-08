package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type TokenType = int

// TokenTypes:
const (
	OpenBrace TokenType = iota
	CloseBrace
	OpenBracket
	CloseBracket
	Quote
	Colon
	Comma
	NumberLiteral
	StringLiteral
)

func tokenTypeToString(t TokenType) string {
	switch t {
	case OpenBrace:
		return "OpenBrace"
	case CloseBrace:
		return "CloseBrace"
	case OpenBracket:
		return "OpenBracket"
	case CloseBracket:
		return "CloseBracket"
	case Quote:
		return "Quote"
	case Colon:
		return "Colon"
	case Comma:
		return "Comma"
	case NumberLiteral:
		return "NumberLiteral"
	case StringLiteral:
		return "StringLiteral"
	}

	panic(fmt.Sprintf("tokenTypeToString: unhandled token type: %v", t))
}

type Token struct {
	TokenType TokenType `json:"type"`
	Lexeme    string    `json:"lexeme"`
}

// I'm probably supposed to use some cool go json tokenizer here or something here so this is actually correct
func tokenize(data []byte) []Token {
	tokens := make([]Token, 0)

	runes := []rune(string(data))
	for i := 0; i < len(runes); i++ {
		char := runes[i]

		if char == '{' {
			tokens = append(tokens, Token{TokenType: OpenBrace, Lexeme: string(char)})
		} else if char == '}' {
			tokens = append(tokens, Token{TokenType: CloseBrace, Lexeme: string(char)})
		} else if char == ']' {
			tokens = append(tokens, Token{TokenType: CloseBracket, Lexeme: string(char)})
		} else if char == '[' {
			tokens = append(tokens, Token{TokenType: OpenBracket, Lexeme: string(char)})
		} else if char == '"' {
			tokens = append(tokens, Token{TokenType: Quote, Lexeme: string(char)})
		} else if char == ':' {
			tokens = append(tokens, Token{TokenType: Colon, Lexeme: string(char)})
		} else if char == ',' {
			tokens = append(tokens, Token{TokenType: Comma, Lexeme: string(char)})
		} else if unicode.IsLetter(char) {
			var lexeme strings.Builder
			for unicode.IsLetter(char) {
				lexeme.WriteRune(char)
				i++
				char = runes[i]
			}

			i--
			tokens = append(tokens, Token{TokenType: StringLiteral, Lexeme: lexeme.String()})
		} else if unicode.IsNumber(char) {
			var lexeme strings.Builder
			// imagine actually supporting JSON number spec KEKW
			for unicode.IsNumber(char) {
				lexeme.WriteRune(char)
				i++
				char = runes[i]
			}

			i--
			tokens = append(tokens, Token{TokenType: NumberLiteral, Lexeme: lexeme.String()})
		}
	}

	return tokens
}

type BtreeJsonParser struct {
	tokens []Token
	idx    int
}

func NewParser(data []byte) *BtreeJsonParser {
	tokens := tokenize(data)
	fmt.Printf("[DEBUG]: tokens=%+v", tokens)
	return &BtreeJsonParser{tokens: tokens, idx: 0}
}

func (parser *BtreeJsonParser) match(tokenType TokenType) (*Token, error) {
	token := parser.peek()
	fmt.Printf("[DEBUG]: match(%s): idx=%d, current_token=%+v\n", tokenTypeToString(tokenType), parser.idx, token)
	if token == nil {
		return nil, fmt.Errorf("unexpected EOF at %d, expected %s", parser.idx, tokenTypeToString(tokenType))
	}

	if token.TokenType != tokenType {
		return nil, fmt.Errorf("invalid token %s at %d, expected %s", token.Lexeme, parser.idx, tokenTypeToString(tokenType))
	}

	parser.idx += 1
	return token, nil
}

func (parser *BtreeJsonParser) peek() *Token {
	if len(parser.tokens) > parser.idx {
		return &parser.tokens[parser.idx]
	}

	return nil
}

type JsonObject = orderedmap.OrderedMap[string, interface{}]

func (parser *BtreeJsonParser) parseKeyValuePair() (string, interface{}, error) {
	key, err := parser.parseString()
	if err != nil {
		return "", nil, err
	}

	if _, err := parser.match(Colon); err != nil {
		return "", nil, err
	}

	value, err := parser.parseValue()

	return key, value, err
}

func (parser *BtreeJsonParser) parseObject() (*JsonObject, error) {
	tree := orderedmap.New[string, interface{}]()

	if _, err := parser.match(OpenBrace); err != nil {
		return nil, err
	}

	lhs, rhs, err := parser.parseKeyValuePair()
	if err != nil {
		return nil, err
	}

	tree.Set(lhs, rhs)

	nextToken := parser.peek()
	for nextToken != nil && nextToken.TokenType == Comma {

		if _, err := parser.match(Comma); err != nil {
			return nil, err
		}

		lhs, rhs, err := parser.parseKeyValuePair()
		if err != nil {
			return nil, err
		}

		tree.Set(lhs, rhs)

		nextToken = parser.peek()
	}

	if _, err = parser.match(CloseBrace); err != nil {
		return nil, err
	}

	return tree, nil
}

func (parser *BtreeJsonParser) parseNumber() (float64, error) {
	token, err := parser.match(NumberLiteral)
	if err != nil {
		return 0.0, nil
	}

	value, err := strconv.ParseFloat(token.Lexeme, 64)
	return value, err
}

func (parser *BtreeJsonParser) parseString() (string, error) {
	if _, err := parser.match(Quote); err != nil {
		return "", err
	}

	token, err := parser.match(StringLiteral)
	if err != nil {
		return "", err
	}

	if _, err := parser.match(Quote); err != nil {
		return "", err
	}

	return token.Lexeme, nil
}

func (parser *BtreeJsonParser) parseArray() ([]interface{}, error) {
	result := make([]interface{}, 0)
	if _, err := parser.match(OpenBracket); err != nil {
		return result, err
	}

	nextToken := parser.peek()
	if nextToken != nil && nextToken.TokenType == CloseBracket {
		return result, nil
	}

	value, err := parser.parseValue()
	if err != nil {
		return result, err
	}

	result = append(result, value)

	nextToken = parser.peek()
	for nextToken != nil && nextToken.TokenType == Comma {
		if _, err := parser.match(Comma); err != nil {
			return result, err
		}

		value, err = parser.parseValue()
		if err != nil {
			return result, err
		}

		result = append(result, value)
		nextToken = parser.peek()
	}

	if _, err := parser.match(CloseBracket); err != nil {
		return result, err
	}

	return result, nil
}

func (parser *BtreeJsonParser) parseValue() (interface{}, error) {
	token := parser.peek()

	if token == nil {
		return nil, nil
	}

	switch token.TokenType {
	case OpenBrace:
		return parser.parseObject()
	case OpenBracket:
		return parser.parseArray()
	case NumberLiteral:
		return parser.parseNumber()
	case Quote:
		return parser.parseString()
	}

	return nil, fmt.Errorf("invalid token at position %d: %s", parser.idx, token.Lexeme)
}

func (parser *BtreeJsonParser) Parse() (*JsonObject, error) {
	if len(parser.tokens) == 0 {
		return nil, nil
	}

	firstToken := parser.tokens[0]
	if firstToken.TokenType != OpenBrace {
		// in the real world we would want a marshal/unmarshal thing that we can reflect on in order to
		// figure out what the "top level object" is supposed to be:
		return nil, fmt.Errorf("invalid opening token for object: %s", tokenTypeToString(firstToken.TokenType))
	}

	tree, err := parser.parseObject()
	return tree, err
}

func bTreeMarshall(tree *JsonObject) (string, error) {
	var result strings.Builder
	result.WriteRune('{')

	errors := make([]error, 0)
	i := 0
	for pair := tree.Oldest(); pair != nil; pair = pair.Next() {
		result.WriteString(fmt.Sprintf("\"%s\": ", pair.Key))
		switch v := pair.Value.(type) {
		case *JsonObject:
			{
				nextResult, err := bTreeMarshall(v)
				if err != nil {
					errors = append(errors, err)
				}

				result.WriteString(nextResult)
			}
		default:
			{
				nextResult, err := json.Marshal(v)

				if err != nil {
					errors = append(errors, err)
				}

				result.WriteString(string(nextResult))
			}
		}

		// write a comma if we are in the last item in the object:
		if i != tree.Len()-1 {
			result.WriteString(", ")
		}

		i++
	}

	result.WriteString("}")

	// idk
	if len(errors) > 0 {
		return "", errors[0]
	}

	return result.String(), nil
}

func main() {
	file, err := os.Open("package.json")

	if err != nil {
		panic(fmt.Errorf("could not open file package.json: %v", err))
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		panic(fmt.Errorf("could not read from file package.json: %v", err))
	}

	parser := NewParser(raw)
	result, err := parser.Parse()

	if err != nil {
		panic(fmt.Errorf("could not parse to btree: %v", err))
	}

	result.Set("custom_key", "some value")

	data, err := bTreeMarshall(result)
	if err != nil {
		panic(fmt.Errorf("could not marshsall btree: %v", err))
	}

	fmt.Printf("[DEBUG]: marshaled final result to: %s\n", string(data))
	err = os.WriteFile("package-new.json", []byte(data), 0644)
	if err != nil {
		panic(fmt.Errorf("could not write to file: %+v", err))
	}

	cmd := exec.Command("prettier", "-w", "package-new.json")
	_, err = cmd.Output()

	if err != nil {
		panic(fmt.Errorf("could not execute prettier on result: %v", err))
	}
}
