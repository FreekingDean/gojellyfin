import type { Schemas } from './api';
import { Pill } from './ui';
import type { Tone } from './ui';

export type How = { kind: string; tone: Tone; change?: string; from?: string; sent?: string; reasons: string[] };

type Stream = Schemas['MediaStream'];

const codecs: Record<string, string> = {
  h264: 'H.264',
  hevc: 'HEVC',
  av1: 'AV1',
  vp8: 'VP8',
  vp9: 'VP9',
  vc1: 'VC-1',
  mpeg2video: 'MPEG-2',
  mpeg4: 'MPEG-4',
  msmpeg4v3: 'DivX',
  aac: 'AAC',
  ac3: 'AC-3',
  eac3: 'E-AC-3',
  dts: 'DTS',
  dca: 'DTS',
  truehd: 'TrueHD',
  mp3: 'MP3',
  opus: 'Opus',
  flac: 'FLAC',
  vorbis: 'Vorbis',
  alac: 'ALAC',
};

const standard = ['SDR', 'DOVIWithSDR', 'Unknown'];

export function codec(name?: string | null): string {
  if (!name) {
    return '';
  }

  return codecs[name.toLowerCase()] ?? name.toUpperCase();
}

function resolution(video?: Stream): string {
  const width = video?.Width ?? 0;
  const height = video?.Height ?? 0;
  if (width >= 3200 || height >= 1800) {
    return '4K';
  }
  if (width >= 1800 || height >= 1000) {
    return '1080p';
  }
  if (width >= 1200 || height >= 700) {
    return '720p';
  }

  return height ? `${height}p` : '';
}

function channels(audio?: Stream): string {
  const count = audio?.Channels ?? 0;
  const named: Record<number, string> = { 1: 'mono', 2: 'stereo', 6: '5.1', 8: '7.1' };

  return named[count] ?? (count ? `${count}ch` : '');
}

function bitrate(bits?: number | null): string {
  if (!bits) {
    return '';
  }

  return bits >= 1_000_000 ? `${(bits / 1_000_000).toFixed(1)} Mbps` : `${Math.round(bits / 1000)} kbps`;
}

function described(source?: Schemas['MediaSourceInfo'], video?: Stream, audio?: Stream): string {
  const range = video?.VideoRangeType && !standard.includes(video.VideoRangeType) ? video.VideoRangeType : '';
  const sound = [codec(audio?.Codec), channels(audio)].filter(Boolean).join(' ');

  return [resolution(video), codec(video?.Codec), range, (source?.Container ?? '').toUpperCase(), sound, bitrate(source?.Bitrate)].filter(Boolean).join(' · ');
}

function spaced(reason: string): string {
  const words = reason.replace(/([a-z])([A-Z])/g, '$1 $2').toLowerCase();

  return words.charAt(0).toUpperCase() + words.slice(1);
}

function reason(name: string, container: string, video?: Stream, audio?: Stream): string {
  switch (name) {
    case 'ContainerNotSupported':
      return container ? `${container} container not supported` : 'Container not supported';
    case 'VideoCodecNotSupported':
      return `${codec(video?.Codec) || 'Video codec'} video not supported`;
    case 'AudioCodecNotSupported':
      return `${codec(audio?.Codec) || 'Audio codec'} audio not supported`;
    case 'VideoRangeTypeNotSupported':
      return `${video?.VideoRangeType ?? 'HDR'} not supported, tone mapped to SDR`;
    case 'VideoProfileNotSupported':
      return video?.Profile ? `${video.Profile} profile not supported` : 'Video profile not supported';
    case 'VideoLevelNotSupported':
      return video?.Level ? `Level ${video.Level} too high` : 'Video level too high';
    case 'InterlacedVideoNotSupported':
      return 'Interlaced, deinterlaced';
    case 'AnamorphicVideoNotSupported':
      return 'Anamorphic, stretched to square pixels';
    case 'VideoResolutionNotSupported':
      return 'Resolution too high';
    case 'VideoBitrateNotSupported':
      return 'Bitrate too high';
    default:
      return spaced(name);
  }
}

export function how(session: Schemas['SessionInfoDto']): How | undefined {
  const item = session.NowPlayingItem;
  if (!item) {
    return undefined;
  }

  const source = item.MediaSources?.[0];
  const streams = item.MediaStreams ?? [];
  const video = streams.find((stream) => stream.Type === 'Video');
  const audio = streams.find((stream) => stream.Type === 'Audio' && stream.Index === source?.DefaultAudioStreamIndex) ?? streams.find((stream) => stream.Type === 'Audio');
  const from = described(source, video, audio) || undefined;

  const info = session.TranscodingInfo;
  if (!info) {
    const method = session.PlayState?.PlayMethod;
    if (method === 'DirectPlay') {
      return { kind: 'Direct play', tone: 'ok', from, sent: source ? 'The file, untouched' : undefined, reasons: [] };
    }

    return method ? { kind: spaced(method), tone: 'off', from, reasons: [] } : undefined;
  }

  const container = (source?.Container ?? '').toUpperCase();
  const reasons = (info.TranscodeReasons ?? []).map((name) => reason(name, container, video, audio));
  const into = source?.TranscodingSubProtocol === 'hls' ? 'HLS' : `${(info.Container ?? '').toUpperCase()} stream`.trim();
  const picture = !video ? '' : info.IsVideoDirect ? `${codec(video.Codec)} copied` : [codec(info.VideoCodec), info.Height ? `${info.Height}p` : ''].filter(Boolean).join(' ');
  const sound = info.IsAudioDirect ? (audio ? `${codec(audio.Codec)} copied` : '') : codec(info.AudioCodec);
  const sent = [into, picture, sound].filter(Boolean).join(' · ');

  if (info.IsVideoDirect && info.IsAudioDirect) {
    return { kind: 'Remux', tone: 'off', from, sent, reasons };
  }
  if (info.IsVideoDirect) {
    return { kind: video ? 'Audio conversion' : 'Audio encode', tone: 'off', from, sent, reasons };
  }
  if (video?.VideoRangeType && !standard.includes(video.VideoRangeType) && !info.TranscodeReasons?.includes('VideoRangeTypeNotSupported')) {
    reasons.push(`${video.VideoRangeType} tone mapped to SDR`);
  }

  return { kind: 'Video encode', tone: 'warn', from, sent, reasons };
}

export function attempted(attempt?: Schemas['ReportAttempt'] | null): How | undefined {
  if (!attempt || attempt.Direct == null) {
    return undefined;
  }

  const reasons = (attempt.Reasons ?? []).map((name) => (name.endsWith('CodecNotSupported') ? spaced(name) : reason(name, '', undefined, undefined)));
  const container = (attempt.Container ?? '').toUpperCase();
  if (attempt.Direct) {
    return { kind: 'Direct play', tone: 'ok', change: container, reasons };
  }
  if (attempt.VideoDirect && attempt.AudioDirect) {
    return { kind: 'Remux', tone: 'off', change: container && `into ${container}`, reasons };
  }

  const into = [attempt.VideoDirect ? '' : codec(attempt.VideoCodec), attempt.AudioDirect ? '' : codec(attempt.AudioCodec), container].filter(Boolean).join(' ');
  const kind = attempt.VideoDirect ? 'Audio conversion' : 'Video encode';

  return { kind, tone: attempt.VideoDirect ? 'off' : 'warn', change: into && `to ${into}`, reasons };
}

export function client(attempt?: Schemas['ReportAttempt'] | null): string | undefined {
  if (!attempt) {
    return undefined;
  }

  const app = [attempt.Client, attempt.ClientVersion].filter(Boolean).join(' ');
  const on = [app, attempt.DeviceName && `on ${attempt.DeviceName}`].filter(Boolean).join(' ');

  return on || undefined;
}

export function Served({ how }: { how?: How }) {
  if (!how) {
    return null;
  }

  return (
    <div class="served">
      <Pill tone={how.tone}>{how.kind}</Pill>
      {how.change && <small>{how.change}</small>}
      {how.reasons.map((line) => (
        <small key={line} class="muted">
          {line}
        </small>
      ))}
    </div>
  );
}

export function Stream({ how }: { how?: How }) {
  if (!how) {
    return null;
  }

  return (
    <>
      {how.from && (
        <small class="stream" title={how.from}>
          <span class="muted">From</span> {how.from}
        </small>
      )}
      {how.sent && (
        <small class="stream" title={how.sent}>
          <span class="muted">Sent</span> {how.sent}
        </small>
      )}
    </>
  );
}
