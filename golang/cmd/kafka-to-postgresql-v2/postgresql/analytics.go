package postgresql

import (
	"github.com/jackc/pgx/v5/pgconn"
	sharedStructs "github.com/united-manufacturing-hub/united-manufacturing-hub/cmd/kafka-to-postgresql-v2/shared"
	"go.uber.org/zap"
)

func (c *Connection) InsertWorkOrderCreate(msg *sharedStructs.WorkOrderCreateMessage, topic *sharedStructs.TopicDetails) error {
	assetId, err := c.GetOrInsertAsset(topic)
	if err != nil {
		return err
	}
	productTypeId, err := c.GetOrInsertProductType(assetId, msg.Product)
	if err != nil {
		return err
	}
	// Start tx (this shouldn't take more then 1 minute)
	ctx, cncl := get1MinuteContext()
	defer cncl()
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}
	// Don't forget to convert unix ms to timestamptz
	var cmdTag pgconn.CommandTag
	cmdTag, err = tx.Exec(ctx, `
		INSERT INTO work_orders (externalWorkOrderId, assetId, productTypeId, quantity, status, to_timestamp($6 / 1000), to_timestamp($7 / 1000))
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, msg.ExternalWorkOrderId, int(assetId), int(productTypeId), msg.Quantity, int(msg.Status), msg.StartTimeUnixMs, msg.EndTimeUnixMs)
	if err != nil {
		zap.S().Warnf("Error inserting work order: %v (workOrderId: %v) [%s]", err, msg.ExternalWorkOrderId, cmdTag)
		errR := tx.Rollback(ctx)
		if errR != nil {
			zap.S().Errorf("Error rolling back transaction: %v", errR)
		}
		return err
	}
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (c *Connection) InsertWorkOrderStart(msg *sharedStructs.WorkOrderStartMessage) error {
	// Update work-order by externalWorkOrderId

	// Start tx (this shouldn't take more then 1 minute)
	ctx, cncl := get1MinuteContext()
	defer cncl()
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}

	var cmdTag pgconn.CommandTag
	cmdTag, err = tx.Exec(ctx, `
		UPDATE work_orders
		SET status = 1, startTime = to_timestamp($2 / 1000)
		WHERE externalWorkOrderId = $1
		  AND status = 0 
		  AND startTime IS NULL
	`, msg.ExternalWorkOrderId, msg.StartTimeUnixMs)
	if err != nil {
		zap.S().Warnf("Error updating work order: %v (workOrderId: %v) [%s]", err, msg.ExternalWorkOrderId, cmdTag)
		errR := tx.Rollback(ctx)
		if errR != nil {
			zap.S().Errorf("Error rolling back transaction: %v", errR)
		}
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (c *Connection) InsertWorkOrderStop(msg *sharedStructs.WorkOrderStopMessage) error {
	// Update work-order by externalWorkOrderId

	// Start tx (this shouldn't take more then 1 minute)
	ctx, cncl := get1MinuteContext()
	defer cncl()
	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}

	var cmdTag pgconn.CommandTag
	cmdTag, err = tx.Exec(ctx, `
		UPDATE work_orders
		SET status = 2, endTime = to_timestamp($2 / 1000)
		WHERE externalWorkOrderId = $1
		  AND status = 1
		  AND endTime IS NULL
	`, msg.ExternalWorkOrderId, msg.EndTimeUnixMs)
	if err != nil {
		zap.S().Warnf("Error updating work order: %v (workOrderId: %v) [%s]", err, msg.ExternalWorkOrderId, cmdTag)
		errR := tx.Rollback(ctx)
		if errR != nil {
			zap.S().Errorf("Error rolling back transaction: %v", errR)
		}
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	return nil
}
