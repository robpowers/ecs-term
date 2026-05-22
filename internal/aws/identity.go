package aws

import (
	"context"
	"fmt"
)

func (c *ClientSet) FetchAccountID(ctx context.Context) (string, error) {
	out, err := c.STS.GetCallerIdentity(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("GetCallerIdentity: %w", err)
	}
	if out.Account == nil {
		return "unknown", nil
	}
	return *out.Account, nil
}
