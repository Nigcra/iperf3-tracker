-- Migration: Add auto-trace functionality
-- Description: Adds auto_trace_enabled column to servers table

ALTER TABLE servers ADD COLUMN auto_trace_enabled BOOLEAN DEFAULT FALSE;
