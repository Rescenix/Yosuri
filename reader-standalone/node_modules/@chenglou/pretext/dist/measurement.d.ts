export type SegmentMetrics = {
    width: number;
    containsCJK: boolean;
    emojiCount?: number;
    breakableFitMode?: BreakableFitMode;
    breakableFitAdvances?: number[] | null;
};
export type EngineProfile = {
    lineFitEpsilon: number;
    carryCJKAfterClosingQuote: boolean;
    breakKeepAllAfterPunctuation: boolean;
    preferPrefixWidthsForBreakableRuns: boolean;
    preferEarlySoftHyphenBreak: boolean;
};
export type BreakableFitMode = 'sum-graphemes' | 'segment-prefixes' | 'pair-context';
export declare function getMeasureContext(): CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;
export declare function getSegmentMetricCache(font: string): Map<string, SegmentMetrics>;
export declare function getSegmentMetrics(seg: string, cache: Map<string, SegmentMetrics>): SegmentMetrics;
export declare function getEngineProfile(): EngineProfile;
export declare function parseFontSize(font: string): number;
export declare function textMayContainEmoji(text: string): boolean;
export declare function getCorrectedSegmentWidth(seg: string, metrics: SegmentMetrics, emojiCorrection: number): number;
export declare function getSegmentBreakableFitAdvances(seg: string, metrics: SegmentMetrics, cache: Map<string, SegmentMetrics>, emojiCorrection: number, mode: BreakableFitMode): number[] | null;
export declare function getFontMeasurementState(font: string, needsEmojiCorrection: boolean): {
    cache: Map<string, SegmentMetrics>;
    fontSize: number;
    emojiCorrection: number;
};
export declare function clearMeasurementCaches(): void;
//# sourceMappingURL=measurement.d.ts.map