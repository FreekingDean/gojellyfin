import type { Schemas } from './api';
import { Pill } from './ui';
import type { Tone } from './ui';

export type How = { kind: string; tone: Tone; change?: string; reasons: string[] };

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

function arrow(from: string, to: string): string | undefined {
  return from && to && from !== to ? `${from} → ${to}` : undefined;
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

  const info = session.TranscodingInfo;
  if (!info) {
    const method = session.PlayState?.PlayMethod;
    if (method === 'DirectPlay') {
      return { kind: 'Direct play', tone: 'ok', reasons: [] };
    }

    return method ? { kind: spaced(method), tone: 'off', reasons: [] } : undefined;
  }

  const source = item.MediaSources?.[0];
  const streams = item.MediaStreams ?? [];
  const video = streams.find((stream) => stream.Type === 'Video');
  const audio = streams.find((stream) => stream.Type === 'Audio' && stream.Index === source?.DefaultAudioStreamIndex) ?? streams.find((stream) => stream.Type === 'Audio');
  const from = (source?.Container ?? '').toUpperCase();
  const reasons = (info.TranscodeReasons ?? []).map((name) => reason(name, from, video, audio));

  if (info.IsVideoDirect && info.IsAudioDirect) {
    return { kind: 'Remux', tone: 'off', change: arrow(from, (info.Container ?? '').toUpperCase()), reasons };
  }
  if (info.IsVideoDirect) {
    return { kind: 'Audio conversion', tone: 'off', change: arrow(codec(audio?.Codec), codec(info.AudioCodec)), reasons };
  }

  const picture = [codec(info.VideoCodec), info.Height && info.Height < (video?.Height ?? 0) ? `${info.Height}p` : ''].filter(Boolean).join(' ');
  const changes = [arrow(codec(video?.Codec) || 'Video', picture)];
  if (!info.IsAudioDirect) {
    changes.push(arrow(codec(audio?.Codec), codec(info.AudioCodec)));
  }
  if (video?.VideoRangeType && !standard.includes(video.VideoRangeType) && !info.TranscodeReasons?.includes('VideoRangeTypeNotSupported')) {
    reasons.push(`${video.VideoRangeType} tone mapped to SDR`);
  }

  return { kind: 'Video encode', tone: 'warn', change: changes.filter(Boolean).join(', '), reasons };
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
