package execute

import (
	"testing"
)

func TestFactory(t *testing.T) {
	factory, err := NewFactory(runnerLimitsForTest())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := factory.Parse(map[string]any{"operations": []any{}})
	if err != nil || len(plan.Operations) != 0 {
		t.Fatal(plan, err)
	}
	clock := func() float64 { return 7 }
	runner := factory.RunnerWithClock(nil, clock)
	if runner.Planner != factory.planner || runner.Arguments != factory.arguments || runner.Now() != 7 {
		t.Fatal(runner)
	}
	if factory.Runner(nil).Now() <= 0 {
		t.Fatal("default clock")
	}
}
func TestFactoryFailures(t *testing.T) {
	oldOps, oldArgs := operationDefinitions, argumentDefinitions
	defer func() { operationDefinitions, argumentDefinitions = oldOps, oldArgs }()
	operationDefinitions = []byte("{")
	if _, err := NewFactory(runnerLimitsForTest()); err == nil {
		t.Fatal("planner")
	}
	operationDefinitions = oldOps
	argumentDefinitions = []byte("{")
	if _, err := NewFactory(runnerLimitsForTest()); err == nil {
		t.Fatal("arguments")
	}
}
