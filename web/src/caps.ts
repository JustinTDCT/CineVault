import type { ClientCaps } from "./api";

// Codec strings the browser is asked about. A codec counts as supported if
// the <video> element or Media Source Extensions can play it.
const VIDEO: Record<string, string> = {
  h264: 'video/mp4; codecs="avc1.640028"',
  hevc: 'video/mp4; codecs="hvc1.1.6.L120.90"',
  av1: 'video/mp4; codecs="av01.0.08M.08"',
  vp9: 'video/webm; codecs="vp9"',
};

const AUDIO: Record<string, string> = {
  aac: 'audio/mp4; codecs="mp4a.40.2"',
  mp3: "audio/mpeg",
  opus: 'audio/webm; codecs="opus"',
  ac3: 'audio/mp4; codecs="ac-3"',
  eac3: 'audio/mp4; codecs="ec-3"',
  flac: 'audio/mp4; codecs="flac"',
};

/** Asks this browser what it can play without conversion. */
export function detectCaps(): ClientCaps {
  const video = document.createElement("video");
  const ok = (type: string) => video.canPlayType(type) !== "" || (window.MediaSource?.isTypeSupported?.(type) ?? false);
  return {
    containers: [ok("video/mp4") && "mp4", ok("video/webm") && "webm"].filter(Boolean) as string[],
    video: Object.keys(VIDEO).filter((k) => ok(VIDEO[k])),
    audio: Object.keys(AUDIO).filter((k) => ok(AUDIO[k])),
  };
}
