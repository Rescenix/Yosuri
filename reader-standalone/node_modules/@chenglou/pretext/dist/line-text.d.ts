import type { PreparedTextWithSegments } from './layout.js';
export declare function getLineTextCache(prepared: PreparedTextWithSegments): Map<number, string[]>;
export declare function buildLineTextFromRange(prepared: PreparedTextWithSegments, cache: Map<number, string[]>, startSegmentIndex: number, startGraphemeIndex: number, endSegmentIndex: number, endGraphemeIndex: number): string;
export declare function clearLineTextCaches(): void;
//# sourceMappingURL=line-text.d.ts.map