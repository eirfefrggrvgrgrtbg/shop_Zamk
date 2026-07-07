package fulfillment

import (
	"errors"
)

var (
	ErrShipmentNotFound         = errors.New("shipment not found")
	ErrFulfillmentNotFound      = errors.New("fulfillment not found")
	ErrOrderNotPaid             = errors.New("shipment can only be created for paid orders")
	ErrShipmentExists           = errors.New("shipment already exists for this order")
	ErrInvalidStatus            = errors.New("invalid shipment status")
	ErrInvalidFulfillmentStatus = errors.New("cannot create shipment for this fulfillment status")
	ErrInvalidTransition        = errors.New("invalid fulfillment status transition")
	ErrUnauthorized             = errors.New("unauthorized")
	ErrMultipleFulfillments     = errors.New("Заказ содержит несколько сборок продавцов. Создайте отгрузку для конкретной сборки.")
)
