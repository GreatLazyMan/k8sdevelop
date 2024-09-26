package constants

import (
  "time"
)

const (
	SimplecontrollerControllerName = "simplecontroller"

	SimplecontrollerFinalizerName   = "GreatLazyMan.io/simplecontroller-finalizer"

  DefaultNamespace = "greatlazyman-system"
  DefaultRequeueTime = 10 * time.Second
)
