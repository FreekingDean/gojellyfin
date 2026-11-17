# Playback

gojellyfin decides how to deliver each item based on the capabilities the client reports in its Jellyfin `DeviceProfile`. The goal is to make the smallest change needed for that client.

Depending on the file and client, playback may be:

- **Direct play** — send the original file without changing it.
- **Remux** — change the container while keeping compatible audio and video streams.
- **Audio conversion** — convert audio the client cannot play.
- **Video encoding** — encode to H.264 at up to 1080p and deliver over HLS.

The server returns one playback URL. The client does not choose among a list of server-side transcode options.

## 4K and source limits

Video encoding is deliberately bounded. By default, a source taller than 1080 pixels is not used as the input to a picture encode. A 4K file can still direct-play or be remuxed if the client supports it; the limit applies when the picture itself must be encoded.

If a title has no copy that the client can play without picture encoding and its only available copy is above the configured source-height limit, playback is refused rather than starting an expensive 4K encode.

With Helm, `transcoderMaxSourceHeight` controls the limit.

## Streaming capacity

Each encode consumes a transcoder slot. The chart's `api.transcoderJobs` and `streaming.transcoderJobs` settings control the number of concurrent jobs per pod; when left empty, the slot count follows the CPU request.

When a pod has no free slots, it can return a 503 so the gateway can try another streaming pod. This is why the streaming workload can be scaled independently from the API.

## Troubleshooting playback

- If a file plays on one device but not another, the clients may report different device profiles.
- If playback is refused, check whether a compatible copy exists and whether it would require encoding above the source-height limit.
- If streams fail under load, check transcoder slot usage and the CPU request for the streaming workload.
- Check the server logs for ffmpeg errors and the gateway's routing to the streaming service.
