package service

type RuntimeCapabilities struct {
	SemanticEnabled, SemanticLoaded bool
	SemanticModelID                 string
	SemanticDimensions              int
	NeedleEnabled, NeedleLoaded     bool
	NeedleModelPath, NeedleRuntime  string
}
