// Package recovery manages cloud database restoration and orphan cleanup.
package recovery

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	TagManaged   = "revenant:managed"
	TagPlan      = "revenant:plan"
	TagCreatedAt = "revenant:created-at"
)

type Client struct {
	client *rds.Client
	region string
}

func NewAWSClient(ctx context.Context, region string) (*Client, error) {
	if err := validateEnvironmentCredentials(); err != nil {
		return nil, err
	}

	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return &Client{
		client: rds.NewFromConfig(cfg),
		region: cfg.Region,
	}, nil
}

func validateEnvironmentCredentials() error {
	accessKey, accessKeySet := os.LookupEnv("AWS_ACCESS_KEY_ID")
	secretKey, secretKeySet := os.LookupEnv("AWS_SECRET_ACCESS_KEY")

	if accessKeySet && (len(accessKey) != 20 || strings.Contains(accessKey, "/")) {
		return fmt.Errorf("AWS_ACCESS_KEY_ID is invalid; use the 20-character access key ID, not the secret access key")
	}
	if secretKeySet && len(secretKey) != 40 {
		return fmt.Errorf("AWS_SECRET_ACCESS_KEY is invalid; use the 40-character secret access key")
	}
	return nil
}

// FindLatestSnapshot looks up the newest manual or automated snapshot for a DB instance.
func (c *Client) FindLatestSnapshot(ctx context.Context, dbInstanceID string) (*types.DBSnapshot, error) {
	output, err := c.client.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
		DBInstanceIdentifier: aws.String(dbInstanceID),
	})
	if err != nil {
		return nil, fmt.Errorf("describe snapshots for %s: %w", dbInstanceID, err)
	}

	if len(output.DBSnapshots) == 0 {
		return nil, fmt.Errorf("no RDS snapshots found for instance %s", dbInstanceID)
	}

	var newest *types.DBSnapshot
	for i := range output.DBSnapshots {
		snap := &output.DBSnapshots[i]
		if snap.Status == nil || *snap.Status != "available" {
			continue
		}
		if newest == nil || (snap.SnapshotCreateTime != nil && snap.SnapshotCreateTime.After(*newest.SnapshotCreateTime)) {
			newest = snap
		}
	}

	if newest == nil {
		return nil, fmt.Errorf("no available RDS snapshot found for instance %s", dbInstanceID)
	}

	return newest, nil
}

// CreateSnapshot creates a manual snapshot of an existing RDS instance.
func (c *Client) CreateSnapshot(ctx context.Context, dbInstanceID string, snapshotID string) error {
	slog.Info("creating RDS snapshot", "instance", dbInstanceID, "snapshot", snapshotID)
	_, err := c.client.CreateDBSnapshot(ctx, &rds.CreateDBSnapshotInput{
		DBInstanceIdentifier: aws.String(dbInstanceID),
		DBSnapshotIdentifier: aws.String(snapshotID),
		Tags:                 []types.Tag{{Key: aws.String(TagManaged), Value: aws.String("true")}},
	})
	if err != nil {
		return fmt.Errorf("create snapshot %s: %w", snapshotID, err)
	}
	return nil
}

// WaitForSnapshot polls until a manual snapshot is available.
func (c *Client) WaitForSnapshot(ctx context.Context, snapshotID string, pollInterval, maxWait time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	if maxWait <= 0 {
		maxWait = 30 * time.Minute
	}
	deadline := time.Now().Add(maxWait)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for snapshot %s to become available (limit: %v)", snapshotID, maxWait)
		}

		output, err := c.client.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
			DBSnapshotIdentifier: aws.String(snapshotID),
		})
		if err == nil && len(output.DBSnapshots) > 0 {
			status := aws.ToString(output.DBSnapshots[0].Status)
			slog.Info("RDS snapshot status", "snapshot", snapshotID, "status", status)
			if status == "available" {
				return nil
			}
		} else if err != nil {
			slog.Warn("error checking RDS snapshot status, retrying", "snapshot", snapshotID, "err", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// RestoreSandbox options for provisioning a temporary DB instance.
type RestoreOptions struct {
	SnapshotARN   string
	SandboxDBID   string
	InstanceClass string
	PlanName      string
}

// RestoreSandbox restores an RDS snapshot into a tagged temporary sandbox instance.
func (c *Client) RestoreSandbox(ctx context.Context, opts RestoreOptions) error {
	instanceClass := opts.InstanceClass
	if instanceClass == "" {
		instanceClass = "db.t4g.micro"
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	input := &rds.RestoreDBInstanceFromDBSnapshotInput{
		DBInstanceIdentifier: aws.String(opts.SandboxDBID),
		DBSnapshotIdentifier: aws.String(opts.SnapshotARN),
		DBInstanceClass:      aws.String(instanceClass),
		PubliclyAccessible:   aws.Bool(true),
		Tags: []types.Tag{
			{Key: aws.String(TagManaged), Value: aws.String("true")},
			{Key: aws.String(TagPlan), Value: aws.String(opts.PlanName)},
			{Key: aws.String(TagCreatedAt), Value: aws.String(nowStr)},
		},
	}

	slog.Info("triggering RDS restore from snapshot", "sandbox_id", opts.SandboxDBID, "snapshot", opts.SnapshotARN)
	_, err := c.client.RestoreDBInstanceFromDBSnapshot(ctx, input)
	if err != nil {
		return fmt.Errorf("restore db from snapshot %s: %w", opts.SnapshotARN, err)
	}

	return nil
}

// DestroySandbox terminates the temporary sandbox DB instance without keeping a final snapshot.
func (c *Client) DestroySandbox(ctx context.Context, sandboxDBID string) error {
	slog.Info("destroying temporary RDS sandbox", "sandbox_id", sandboxDBID)
	_, err := c.client.DeleteDBInstance(ctx, &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier: aws.String(sandboxDBID),
		SkipFinalSnapshot:    aws.Bool(true),
	})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return nil
		}
		return fmt.Errorf("delete rds sandbox %s: %w", sandboxDBID, err)
	}

	return nil
}

// WaitForDB polls the RDS instance until it is available for connections.
// Timeout is typically 15-30 minutes for a restore operation.
func (c *Client) WaitForDB(ctx context.Context, sandboxDBID string, pollInterval time.Duration, maxWait time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 30 * time.Second
	}
	if maxWait <= 0 {
		maxWait = 30 * time.Minute
	}

	deadline := time.Now().Add(maxWait)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for RDS instance %s to be available (limit: %v)", sandboxDBID, maxWait)
		}

		output, err := c.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
			DBInstanceIdentifier: aws.String(sandboxDBID),
		})
		if err != nil {
			slog.Warn("error checking RDS status, retrying", "err", err)
			time.Sleep(pollInterval)
			continue
		}

		if len(output.DBInstances) == 0 {
			slog.Warn("RDS instance not found, retrying", "instance", sandboxDBID)
			time.Sleep(pollInterval)
			continue
		}

		instance := &output.DBInstances[0]
		status := aws.ToString(instance.DBInstanceStatus)
		slog.Info("RDS instance status", "instance", sandboxDBID, "status", status)

		if status == "available" {
			return nil
		}

		time.Sleep(pollInterval)
	}
}

// GetEndpoint extracts the Postgres connection endpoint from an RDS instance.
// Returns host:port for use in connection strings.
func (c *Client) GetEndpoint(ctx context.Context, sandboxDBID string) (string, error) {
	output, err := c.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(sandboxDBID),
	})
	if err != nil {
		return "", fmt.Errorf("describe db instance %s: %w", sandboxDBID, err)
	}

	if len(output.DBInstances) == 0 {
		return "", fmt.Errorf("RDS instance %s not found", sandboxDBID)
	}

	instance := &output.DBInstances[0]
	if instance.Endpoint == nil {
		return "", fmt.Errorf("RDS instance %s has no endpoint", sandboxDBID)
	}

	host := aws.ToString(instance.Endpoint.Address)
	port := aws.ToInt32(instance.Endpoint.Port)
	if host == "" || port == 0 {
		return "", fmt.Errorf("RDS instance %s endpoint is invalid (host=%s, port=%d)", sandboxDBID, host, port)
	}

	endpoint := fmt.Sprintf("%s:%d", host, port)
	slog.Info("extracted RDS endpoint", "instance", sandboxDBID, "endpoint", endpoint)
	return endpoint, nil
}
