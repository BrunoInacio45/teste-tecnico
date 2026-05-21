package worker

import (
	"context"
	"sync"
)

// Job representa uma mensagem SQS a ser processada.
type Job struct {
	Body          string
	ReceiptHandle string
}

// Start cria n goroutines que lêem de jobs e chamam fn para cada uma.
// Bloqueia até que todas as goroutines terminem.
// As goroutines param quando jobs é fechado ou ctx é cancelado.
func Start(ctx context.Context, n int, jobs <-chan Job, fn func(context.Context, Job)) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case j, ok := <-jobs:
					if !ok {
						return // canal fechado: worker encerra
					}
					fn(ctx, j)
				case <-ctx.Done():
					return // shutdown solicitado
				}
			}
		}()
	}
	wg.Wait()
}
