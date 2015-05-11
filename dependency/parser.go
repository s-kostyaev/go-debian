/* Copyright (c) Paul R. Tagliamonte <paultag@debian.org>, 2015
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE. */

package dependency

import (
	"errors"
	"fmt"
)

// Parse a string into a Depedency object. The input should look something
// like "foo, bar | baz".
func Parse(in string) (*Depedency, error) {
	ibuf := Input{Index: 0, Data: in}
	dep := &Depedency{Relations: []*Relation{}}
	err := parseDependency(&ibuf, dep)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

/*
 */
type Input struct {
	Data  string
	Index int
}

/*
 */
func (i *Input) Peek() byte {
	if (i.Index) >= len(i.Data) {
		return 0
	}
	return i.Data[i.Index]
}

/*
 */
func (i *Input) Next() byte {
	chr := i.Peek()
	i.Index++
	return chr
}

/* */
func eatWhitespace(input *Input) {
	for {
		peek := input.Peek()
		switch peek {
		case '\r', '\n', ' ', '\t':
			input.Next()
			continue
		}
		break
	}
}

/* */
func parseDependency(input *Input, ret *Depedency) error {
	eatWhitespace(input)

	for {
		peek := input.Peek()
		switch peek {
		case 0: /* EOF, yay */
			return nil
		case ',': /* Next relation set */
			input.Next()
			eatWhitespace(input)
			continue
		}
		err := parseRelation(input, ret)
		if err != nil {
			return err
		}
	}
}

/* */
func parseRelation(input *Input, dependency *Depedency) error {
	eatWhitespace(input) /* Clean out leading whitespace */

	ret := &Relation{Possibilities: []*Possibility{}}

	for {
		peek := input.Peek()
		switch peek {
		case 0, ',': /* EOF, or done with this relation! yay */
			dependency.Relations = append(dependency.Relations, ret)
			return nil
		case '|': /* Next Possi */
			input.Next()
			eatWhitespace(input)
			continue
		}
		err := parsePossibility(input, ret)
		if err != nil {
			return err
		}
	}
}

/* */
func parsePossibility(input *Input, relation *Relation) error {
	eatWhitespace(input) /* Clean out leading whitespace */
	ret := &Possibility{
		Name:    "",
		Version: nil,
		Arches:  &ArchSet{Arches: []*Arch{}},
		Stages:  &StageSet{Stages: []*Stage{}},
	}

	for {
		peek := input.Peek()
		switch peek {
		case ' ':
			err := parsePossibilityControllers(input, ret)
			if err != nil {
				return err
			}
			continue
		case ',', '|', 0: /* I'm out! */
			if ret.Name == "" {
				return errors.New("No package name in Possibility")
			}
			relation.Possibilities = append(relation.Possibilities, ret)
			return nil
		}
		/* Not a control, let's append */
		ret.Name += string(input.Next())
	}
}

/* */
func parsePossibilityControllers(input *Input, possi *Possibility) error {
	for {
		eatWhitespace(input) /* Clean out leading whitespace */
		peek := input.Peek()
		switch peek {
		case ',', '|', 0:
			return nil
		case '(':
			if possi.Version != nil {
				return errors.New(
					"Only one Version relation per Possibility, please!",
				)
			}
			err := parsePossibilityVersion(input, possi)
			if err != nil {
				return err
			}
			continue
		case '[':
			if len(possi.Arches.Arches) != 0 {
				return errors.New(
					"Only one Arch relation per Possibility, please!",
				)
			}
			err := parsePossibilityArchs(input, possi)
			if err != nil {
				return err
			}
			continue
		}
		return fmt.Errorf("Trailing garbage in a Possibility: %c", peek)
	}
	return nil
}

/* */
func parsePossibilityVersion(input *Input, possi *Possibility) error {
	eatWhitespace(input)
	input.Next() /* mandated to be ( */
	// assert ch == '('
	version := VersionRelation{}

	err := parsePossibilityOperator(input, &version)
	if err != nil {
		return err
	}

	err = parsePossibilityNumber(input, &version)
	if err != nil {
		return err
	}

	input.Next() /* OK, let's tidy up */
	// assert ch == ')'

	possi.Version = &version
	return nil
}

/* */
func parsePossibilityOperator(input *Input, version *VersionRelation) error {
	eatWhitespace(input)
	leader := input.Next() /* may be 0 */

	if leader == '=' {
		/* Great, good enough. */
		version.Operator = "="
		return nil
	}

	/* This is always one of:
	 * >=, <=, <<, >> */
	secondary := input.Next()
	if leader == 0 || secondary == 0 {
		return errors.New("Oh no. Reached EOF before Operator finished")
	}

	operator := string([]rune{rune(leader), rune(secondary)})

	switch operator {
	case ">=", "<=", "<<", ">>":
		version.Operator = operator
		return nil
	}

	return fmt.Errorf(
		"Unknown Operator in Possibility Version modifier: %s",
		operator,
	)

}

/* */
func parsePossibilityNumber(input *Input, version *VersionRelation) error {
	eatWhitespace(input)
	for {
		peek := input.Peek()
		switch peek {
		case 0:
			return errors.New("Oh no. Reached EOF before Number finished")
		case ')':
			return nil
		}
		version.Number += string(input.Next())
	}
}

/* */
func parsePossibilityArchs(input *Input, possi *Possibility) error {
	eatWhitespace(input)
	input.Next() /* Assert ch == '[' */

	/* So the first line of each guy can be a not (!), so let's check for
	 * that with a Peek :) */
	peek := input.Peek()
	if peek == '!' {
		input.Next() /* Omnom */
		possi.Arches.Not = true
	}

	for {
		peek := input.Peek()
		switch peek {
		case 0:
			return errors.New("Oh no. Reached EOF before Arch list finished")
		case ']':
			input.Next()
			return nil
		}

		err := parsePossibilityArch(input, possi)
		if err != nil {
			return err
		}
	}
}

/* */
func parsePossibilityArch(input *Input, possi *Possibility) error {
	eatWhitespace(input)
	arch := ""

	for {
		peek := input.Peek()
		switch peek {
		case 0:
			return errors.New("Oh no. Reached EOF before Arch list finished")
		case '!':
			return errors.New("You can only negate whole blocks :(")
		case ']', ' ': /* Let our parent deal with both of these */
			archObj, err := ParseArch(arch)
			if err != nil {
				return err
			}
			possi.Arches.Arches = append(possi.Arches.Arches, archObj)
			return nil
		}
		arch += string(input.Next())
	}
}