package bdd

import "testing"

// TestFeatures ejecuta cada escenario de /features como un subtest.
func TestFeatures(t *testing.T) {
	results, err := Run("../../features")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 20 {
		t.Fatalf("se esperaban >= 20 escenarios, hay %d", len(results))
	}
	for _, r := range results {
		r := r
		t.Run(r.Feature+"/"+r.Scenario.Name, func(t *testing.T) {
			if !r.Passed {
				for _, s := range r.Steps {
					if s.Err != nil {
						t.Errorf("línea %d «%s»: %v", s.Step.Line, s.Step.Text, s.Err)
					}
				}
			}
		})
	}
}
