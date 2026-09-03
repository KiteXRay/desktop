package networkready

import "context"

func WaitUntilReady(ctx context.Context) bool {
	return waitUntilReadyOS(ctx)
}
