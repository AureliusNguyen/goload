export const NEAR_BOTTOM_PX = 80;

export interface ScrollMetrics {
  scrollTop: number;
  scrollHeight: number;
  clientHeight: number;
}

/** True when the reader is at, or within `thresholdPx` of, the end of the pane. */
export function isNearBottom(metrics: ScrollMetrics, thresholdPx: number = NEAR_BOTTOM_PX): boolean {
  return metrics.scrollHeight - metrics.scrollTop - metrics.clientHeight <= thresholdPx;
}
