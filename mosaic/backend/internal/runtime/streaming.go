package runtime

import "mosaic/internal/schema"

// ChunkReader feeds rows to a processing function in fixed-size chunks
// rather than materializing an entire dataset, which is what lets MOSAIC
// process multi-gigabyte CSV/log files without exhausting RAM. Backpressure
// comes for free: the producer blocks on the channel send until the
// consumer (Process) drains the previous chunk.
type ChunkReader struct {
	ChunkSize int
	buffer    []schema.Row
	sink      chan []schema.Row
}

// NewChunkReader creates a reader that batches rows into chunks of size n
// (default 2000 when n<=0) before handing them to the pipeline stage.
func NewChunkReader(n int) *ChunkReader {
	if n <= 0 {
		n = 2000
	}
	return &ChunkReader{ChunkSize: n, sink: make(chan []schema.Row, 4)}
}

// Push adds a row to the current chunk, flushing to the (buffered, hence
// backpressured) sink channel once the chunk fills.
func (c *ChunkReader) Push(r schema.Row) {
	c.buffer = append(c.buffer, r)
	if len(c.buffer) >= c.ChunkSize {
		c.flush()
	}
}

func (c *ChunkReader) flush() {
	if len(c.buffer) == 0 {
		return
	}
	c.sink <- c.buffer
	c.buffer = nil
}

// Close flushes any remaining partial chunk and closes the sink channel.
// Must be called exactly once after the last Push.
func (c *ChunkReader) Close() {
	c.flush()
	close(c.sink)
}

// Chunks exposes the receive-only channel for a consumer's `for range`
// loop, e.g. `for chunk := range reader.Chunks() { process(chunk) }`.
func (c *ChunkReader) Chunks() <-chan []schema.Row { return c.sink }
