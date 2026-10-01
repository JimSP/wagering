//go:build integration

// Package concurrency: cenários multi-instância (>=3 processos), 50 envios paralelos,
// disputa 2x80.00 sobre 100.00, publishers concorrentes, reentrega SQS, reinício.
package concurrency
