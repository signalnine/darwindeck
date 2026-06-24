// Command grammar-judge closes the discovery loop: it emits BLIND novelty
// dossiers for the distinct grammar compositions (one per well-typed family),
// keyed on GameSpec.Composition, so an out-of-loop judge can score them and fill
// the JudgeVerdicts table the novelty-select GA (cmd/grammar-evolve) reads. The
// same pkg/judge apparatus v2 uses, now over the typed grammar space.
//
//	grammar-judge emit -out <dir>     # write dossiers + manifest + prompt + answer key
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darwindeck/darwindeck/pkg/grammar"
	"github.com/darwindeck/darwindeck/pkg/judge"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "emit" {
		fmt.Fprintln(os.Stderr, "usage: grammar-judge emit [-out <dir>]")
		os.Exit(2)
	}
	out := "output/grammar-judge"
	for i := 2; i < len(os.Args)-1; i++ {
		if os.Args[i] == "-out" {
			out = os.Args[i+1]
		}
	}

	specs := grammar.EnumerateModified() // one representative per well-typed family
	res, err := judge.EmitGrammar(specs, filepath.Join(out, "dossiers"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "emit:", err)
		os.Exit(1)
	}

	// The answer key is PRIVATE -- written OUTSIDE the dossier dir so it never
	// leaks into the blind set. true_name is the composition key.
	keyPath := filepath.Join(out, "answer-key.json")
	data, _ := json.MarshalIndent(res.AnswerKey, "", "  ")
	if err := os.WriteFile(keyPath, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "answer key:", err)
		os.Exit(1)
	}

	fmt.Printf("emitted %d blind dossiers to %s\n", len(res.IDs), res.DossierDir)
	fmt.Printf("answer key (id -> composition) -> %s\n", keyPath)
	fmt.Printf("judge the dossiers, then fold verdicts into a Composition-keyed JudgeVerdicts table.\n")
}
