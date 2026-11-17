SET LOCAL lock_timeout = '30s';
SET LOCAL client_connection_check_interval = '10s';
ALTER TABLE "playlist_entries" DROP CONSTRAINT IF EXISTS "playlist_entry_sort_order_not_negative", ADD CONSTRAINT "playlist_entry_sort_order_not_negative" CHECK (sort_order >= 0);
