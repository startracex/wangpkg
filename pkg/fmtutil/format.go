package fmtutil

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

var (
	defaults = &Definition{
		IndentStyle:        editorconfig.IndentStyleSpaces,
		IndentSize:         "4",
		EndOfLine:          RuntimeEOL(),
		InsertFinalNewline: new(true),
	}

	cachedParser = editorconfig.NewCachedParser()
)

type Definition editorconfig.Definition

func (f *Definition) replaceEndLines(def *Definition, content []byte) []byte {
	eol := f.EndOfLine
	if def != nil && def.EndOfLine != "" {
		eol = def.EndOfLine
	}
	return ReplaceEndLines(content, eol)
}

func (f *Definition) insertFinalNewline(def *Definition, content []byte) []byte {
	var insert *bool
	if def != nil && def.InsertFinalNewline != nil {
		insert = def.InsertFinalNewline
	} else {
		insert = f.InsertFinalNewline
	}
	if insert == nil {
		return content
	}

	eol := f.EndOfLine
	if def != nil && def.EndOfLine != "" {
		eol = def.EndOfLine
	}

	if *insert {
		return InsertFinalNewline(content, eol)
	}
	return RemoveFinalNewLine(content)
}

func (f *Definition) Indent(def *Definition) string {
	size := f.IndentSize
	style := f.IndentStyle
	if def != nil {
		if def.IndentSize != "" {
			size = def.IndentSize
		}
		if def.IndentStyle != "" {
			style = def.IndentStyle
		}
	}
	return Indent(size, style)
}

func (f *Definition) Format(fileName string, content []byte) []byte {
	def := GetDefinition(fileName)
	content = NormalizeToLF(content)
	content = f.replaceEndLines(def, content)
	content = f.insertFinalNewline(def, content)
	return content
}

func (f *Definition) FormatJSON(fileName string, in any, opts ...jsontext.Options) ([]byte, error) {
	def := GetDefinition(fileName)
	indent := f.Indent(def)
	opts = append([]jsontext.Options{jsontext.WithIndent(indent)}, opts...)
	content, err := json.Marshal(in, opts...)
	if err != nil {
		return nil, err
	}
	content = f.replaceEndLines(def, content)
	content = f.insertFinalNewline(def, content)
	return content, nil
}

func GetDefinition(name string) *Definition {
	if name == "" {
		return nil
	}
	c := new(editorconfig.Config)
	c.Parser = cachedParser
	def, err := c.Load(name)
	if err != nil {
		return nil
	}
	return (*Definition)(def)
}

func Format(fileName string, content []byte) []byte {
	return defaults.Format(fileName, content)
}

func FormatJSON(fileName string, in any, opts ...jsontext.Options) ([]byte, error) {
	return defaults.FormatJSON(fileName, in, opts...)
}
