package execute

import "time"

// Factory owns immutable plan and argument contracts shared by request runners.
// Each Runner still owns only request-local dispatch/clock configuration.
type Factory struct {
	planner   *Planner
	arguments *Arguments
	limits    Limits
}

func NewFactory(limits Limits) (*Factory, error) {
	planner, err := NewPlanner()
	if err != nil {
		return nil, err
	}
	arguments, err := NewArguments()
	if err != nil {
		return nil, err
	}
	return &Factory{planner: planner, arguments: arguments, limits: limits}, nil
}
func (f *Factory) Parse(value any) (Plan, error) { return f.planner.ParsePlan(value) }
func (f *Factory) Runner(dispatch Dispatcher) *Runner {
	return f.RunnerWithClock(dispatch, func() float64 { return float64(time.Now().UnixNano()) / 1e9 })
}
func (f *Factory) RunnerWithClock(dispatch Dispatcher, clock func() float64) *Runner {
	return &Runner{Planner: f.planner, Arguments: f.arguments, Limits: f.limits, Dispatch: dispatch, Now: clock}
}
