package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

type ReapSummary struct {
	InspectedCount int
	OrphansFound   int
	ReapedIDs      []string
	Errors         []string
}

// ReapOrphans scans the AWS account for RDS instances tagged `revenant:managed=true`.
// If an instance is older than maxAge (default: 2h), it issues a DeleteDBInstance call.
func (c *Client) ReapOrphans(ctx context.Context, maxAge time.Duration) (*ReapSummary, error) {
	if maxAge <= 0 {
		maxAge = 2 * time.Hour
	}

	summary := &ReapSummary{}
	now := time.Now().UTC()

	paginator := rds.NewDescribeDBInstancesPaginator(c.client, &rds.DescribeDBInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list rds instances: %w", err)
		}

		for _, instance := range page.DBInstances {
			summary.InspectedCount++
			dbID := aws.ToString(instance.DBInstanceIdentifier)

			isManaged := false
			var createdAt time.Time

			for _, tag := range instance.TagList {
				key := aws.ToString(tag.Key)
				val := aws.ToString(tag.Value)

				if key == TagManaged && val == "true" {
					isManaged = true
				}
				if key == TagCreatedAt {
					if t, err := time.Parse(time.RFC3339, val); err == nil {
						createdAt = t
					}
				}
			}

			if !isManaged {
				continue
			}

			// Fallback to InstanceCreateTime if tag parsing failed
			if createdAt.IsZero() && instance.InstanceCreateTime != nil {
				createdAt = *instance.InstanceCreateTime
			}

			age := now.Sub(createdAt)
			if age > maxAge {
				summary.OrphansFound++
				slog.Warn("found orphaned revenant sandbox instance", "id", dbID, "age", age.Round(time.Minute))

				if err := c.DestroySandbox(ctx, dbID); err != nil {
					summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %v", dbID, err))
				} else {
					summary.ReapedIDs = append(summary.ReapedIDs, dbID)
				}
			}
		}
	}

	return summary, nil
}
