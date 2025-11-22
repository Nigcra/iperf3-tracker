-- Add geoip_interpolated field to trace_hops table
ALTER TABLE trace_hops ADD COLUMN geoip_interpolated BOOLEAN DEFAULT FALSE;
