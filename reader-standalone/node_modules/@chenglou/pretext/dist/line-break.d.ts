import type { SegmentBreakKind } from './analysis.js';
export type LineBreakCursor = {
    segmentIndex: number;
    graphemeIndex: number;
};
export type PreparedLineBreakData = {
    widths: number[];
    lineEndFitAdvances: number[];
    lineEndPaintAdvances: number[];
    kinds: SegmentBreakKind[];
    simpleLineWalkFastPath: boolean;
    breakableFitAdvances: (number[] | null)[];
    breakablePreferredBreaks: (number[] | null)[];
    letterSpacing: number;
    spacingGraphemeCounts: number[];
    discretionaryHyphenWidth: number;
    tabStopAdvance: number;
    chunks: {
        startSegmentIndex: number;
        endSegmentIndex: number;
        consumedEndSegmentIndex: number;
    }[];
};
type InternalLineVisitor = (width: number, startSegmentIndex: number, startGraphemeIndex: number, endSegmentIndex: number, endGraphemeIndex: number) => void;
export declare function normalizePreparedLineStart(prepared: PreparedLineBreakData, cursor: LineBreakCursor): number;
export declare function countPreparedLines(prepared: PreparedLineBreakData, maxWidth: number): number;
export declare function walkPreparedLinesRaw(prepared: PreparedLineBreakData, maxWidth: number, onLine?: InternalLineVisitor): number;
export declare function stepPreparedLineGeometryFromChunk(prepared: PreparedLineBreakData, cursor: LineBreakCursor, chunkIndex: number, maxWidth: number): number | null;
export declare function stepPreparedLineGeometry(prepared: PreparedLineBreakData, cursor: LineBreakCursor, maxWidth: number): number | null;
export declare function measurePreparedLineGeometry(prepared: PreparedLineBreakData, maxWidth: number): {
    lineCount: number;
    maxLineWidth: number;
};
export {};
//# sourceMappingURL=line-break.d.ts.map