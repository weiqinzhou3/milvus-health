package probes

import "context"

type BusinessReadProbe interface {
	Ping(ctx context.Context) error
}

type RWProbe interface {
	Ping(ctx context.Context) error
}
