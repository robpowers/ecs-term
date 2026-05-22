package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/robpowers/ecs-term/internal/config"
)

type ClientSet struct {
	ECS    *ecs.Client
	CWLogs *cloudwatchlogs.Client
	STS    *sts.Client
	Cfg    aws.Config
}

func NewClientSet(ctx context.Context, appCtx config.Context) (*ClientSet, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithSharedConfigProfile(appCtx.AWSProfile),
		awsconfig.WithRegion(appCtx.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config for profile %q: %w", appCtx.AWSProfile, err)
	}
	return &ClientSet{
		ECS:    ecs.NewFromConfig(cfg),
		CWLogs: cloudwatchlogs.NewFromConfig(cfg),
		STS:    sts.NewFromConfig(cfg),
		Cfg:    cfg,
	}, nil
}
