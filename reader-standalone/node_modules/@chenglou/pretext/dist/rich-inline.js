import { measureNaturalWidth, prepareWithSegments, } from './layout.js';
import { buildLineTextFromRange, getLineTextCache, } from './line-text.js';
import { stepPreparedLineGeometry, } from './line-break.js';
const COLLAPSIBLE_BOUNDARY_RE = /[ \t\n\f\r]+/;
const LEADING_COLLAPSIBLE_BOUNDARY_RE = /^[ \t\n\f\r]+/;
const TRAILING_COLLAPSIBLE_BOUNDARY_RE = /[ \t\n\f\r]+$/;
const EMPTY_LAYOUT_CURSOR = { segmentIndex: 0, graphemeIndex: 0 };
const RICH_INLINE_START_CURSOR = {
    itemIndex: 0,
    segmentIndex: 0,
    graphemeIndex: 0,
};
function getInternalPreparedRichInline(prepared) {
    return prepared;
}
function cloneCursor(cursor) {
    return {
        segmentIndex: cursor.segmentIndex,
        graphemeIndex: cursor.graphemeIndex,
    };
}
function isLineStartCursor(cursor) {
    return cursor.segmentIndex === 0 && cursor.graphemeIndex === 0;
}
function getCollapsedSpaceWidth(font, letterSpacing, cache) {
    const cacheKey = `${font}\u0000${letterSpacing}`;
    const cached = cache.get(cacheKey);
    if (cached !== undefined)
        return cached;
    const options = letterSpacing === 0 ? undefined : { letterSpacing };
    const joinedWidth = measureNaturalWidth(prepareWithSegments('A A', font, options));
    const compactWidth = measureNaturalWidth(prepareWithSegments('AA', font, options));
    const collapsedWidth = Math.max(0, joinedWidth - compactWidth);
    cache.set(cacheKey, collapsedWidth);
    return collapsedWidth;
}
function prepareWholeItemLine(prepared) {
    const end = { segmentIndex: 0, graphemeIndex: 0 };
    const width = stepPreparedLineGeometry(prepared, end, Number.POSITIVE_INFINITY);
    if (width === null)
        return null;
    return {
        endGraphemeIndex: end.graphemeIndex,
        endSegmentIndex: end.segmentIndex,
        width,
    };
}
function endsInsideFirstSegment(segmentIndex, graphemeIndex) {
    return segmentIndex === 0 && graphemeIndex > 0;
}
export function prepareRichInline(items) {
    const preparedItems = [];
    const itemsBySourceItemIndex = Array.from({ length: items.length });
    const collapsedSpaceWidthCache = new Map();
    let pendingGapWidth = 0;
    for (let index = 0; index < items.length; index++) {
        const item = items[index];
        const letterSpacing = item.letterSpacing ?? 0;
        const hasLeadingWhitespace = LEADING_COLLAPSIBLE_BOUNDARY_RE.test(item.text);
        const hasTrailingWhitespace = TRAILING_COLLAPSIBLE_BOUNDARY_RE.test(item.text);
        const trimmedText = item.text
            .replace(LEADING_COLLAPSIBLE_BOUNDARY_RE, '')
            .replace(TRAILING_COLLAPSIBLE_BOUNDARY_RE, '');
        if (trimmedText.length === 0) {
            if (COLLAPSIBLE_BOUNDARY_RE.test(item.text) && pendingGapWidth === 0) {
                pendingGapWidth = getCollapsedSpaceWidth(item.font, letterSpacing, collapsedSpaceWidthCache);
            }
            continue;
        }
        const gapBefore = pendingGapWidth > 0
            ? pendingGapWidth
            : hasLeadingWhitespace
                ? getCollapsedSpaceWidth(item.font, letterSpacing, collapsedSpaceWidthCache)
                : 0;
        const prepared = prepareWithSegments(trimmedText, item.font, letterSpacing === 0 ? undefined : { letterSpacing });
        const wholeLine = prepareWholeItemLine(prepared);
        if (wholeLine === null) {
            pendingGapWidth = hasTrailingWhitespace
                ? getCollapsedSpaceWidth(item.font, letterSpacing, collapsedSpaceWidthCache)
                : 0;
            continue;
        }
        const preparedItem = {
            break: item.break ?? 'normal',
            endGraphemeIndex: wholeLine.endGraphemeIndex,
            endSegmentIndex: wholeLine.endSegmentIndex,
            extraWidth: item.extraWidth ?? 0,
            gapBefore,
            naturalWidth: wholeLine.width,
            prepared,
            sourceItemIndex: index,
        };
        preparedItems.push(preparedItem);
        itemsBySourceItemIndex[index] = preparedItem;
        pendingGapWidth = hasTrailingWhitespace
            ? getCollapsedSpaceWidth(item.font, letterSpacing, collapsedSpaceWidthCache)
            : 0;
    }
    return {
        items: preparedItems,
        itemsBySourceItemIndex,
    };
}
function stepRichInlineLine(flow, maxWidth, cursor, collectFragment) {
    if (flow.items.length === 0 || cursor.itemIndex >= flow.items.length)
        return null;
    const safeWidth = Math.max(1, maxWidth);
    let lineWidth = 0;
    let remainingWidth = safeWidth;
    let itemIndex = cursor.itemIndex;
    lineLoop: while (itemIndex < flow.items.length) {
        const item = flow.items[itemIndex];
        if (!isLineStartCursor(cursor) &&
            cursor.segmentIndex === item.endSegmentIndex &&
            cursor.graphemeIndex === item.endGraphemeIndex) {
            itemIndex++;
            cursor.segmentIndex = 0;
            cursor.graphemeIndex = 0;
            continue;
        }
        const gapBefore = lineWidth === 0 ? 0 : item.gapBefore;
        const atItemStart = isLineStartCursor(cursor);
        if (item.break === 'never') {
            if (!atItemStart) {
                itemIndex++;
                cursor.segmentIndex = 0;
                cursor.graphemeIndex = 0;
                continue;
            }
            const occupiedWidth = item.naturalWidth + item.extraWidth;
            const totalWidth = gapBefore + occupiedWidth;
            if (lineWidth > 0 && totalWidth > remainingWidth)
                break lineLoop;
            collectFragment?.(item, gapBefore, occupiedWidth, cloneCursor(EMPTY_LAYOUT_CURSOR), {
                segmentIndex: item.endSegmentIndex,
                graphemeIndex: item.endGraphemeIndex,
            });
            lineWidth += totalWidth;
            remainingWidth = Math.max(0, safeWidth - lineWidth);
            itemIndex++;
            cursor.segmentIndex = 0;
            cursor.graphemeIndex = 0;
            continue;
        }
        const reservedWidth = gapBefore + item.extraWidth;
        if (lineWidth > 0 && reservedWidth >= remainingWidth)
            break lineLoop;
        if (atItemStart) {
            const totalWidth = reservedWidth + item.naturalWidth;
            if (totalWidth <= remainingWidth) {
                collectFragment?.(item, gapBefore, item.naturalWidth + item.extraWidth, cloneCursor(EMPTY_LAYOUT_CURSOR), {
                    segmentIndex: item.endSegmentIndex,
                    graphemeIndex: item.endGraphemeIndex,
                });
                lineWidth += totalWidth;
                remainingWidth = Math.max(0, safeWidth - lineWidth);
                itemIndex++;
                cursor.segmentIndex = 0;
                cursor.graphemeIndex = 0;
                continue;
            }
        }
        const availableWidth = Math.max(1, remainingWidth - reservedWidth);
        const lineEnd = {
            segmentIndex: cursor.segmentIndex,
            graphemeIndex: cursor.graphemeIndex,
        };
        const lineWidthForItem = stepPreparedLineGeometry(item.prepared, lineEnd, availableWidth);
        if (lineWidthForItem === null) {
            itemIndex++;
            cursor.segmentIndex = 0;
            cursor.graphemeIndex = 0;
            continue;
        }
        if (cursor.segmentIndex === lineEnd.segmentIndex &&
            cursor.graphemeIndex === lineEnd.graphemeIndex) {
            itemIndex++;
            cursor.segmentIndex = 0;
            cursor.graphemeIndex = 0;
            continue;
        }
        const itemOccupiedWidth = lineWidthForItem + item.extraWidth;
        const lineWidthContribution = gapBefore + itemOccupiedWidth;
        // The lower-level walker may force one unit to make progress. If that unit
        // only fits on a fresh line, wrap before this rich item instead.
        if (lineWidth > 0 && atItemStart && lineWidthContribution > remainingWidth)
            break lineLoop;
        // If the only thing we can fit after paying the boundary gap is a partial
        // slice of the item's first segment, prefer wrapping before the item so we
        // keep whole-word-style boundaries when they exist. But once the current
        // line can consume a real breakable unit from the item, stay greedy and
        // keep filling the line.
        if (lineWidth > 0 &&
            atItemStart &&
            gapBefore > 0 &&
            endsInsideFirstSegment(lineEnd.segmentIndex, lineEnd.graphemeIndex)) {
            const freshLineEnd = { segmentIndex: 0, graphemeIndex: 0 };
            const freshLineWidth = stepPreparedLineGeometry(item.prepared, freshLineEnd, Math.max(1, safeWidth - item.extraWidth));
            if (freshLineWidth !== null &&
                (freshLineEnd.segmentIndex > lineEnd.segmentIndex ||
                    (freshLineEnd.segmentIndex === lineEnd.segmentIndex &&
                        freshLineEnd.graphemeIndex > lineEnd.graphemeIndex))) {
                break lineLoop;
            }
        }
        collectFragment?.(item, gapBefore, itemOccupiedWidth, cloneCursor(cursor), {
            segmentIndex: lineEnd.segmentIndex,
            graphemeIndex: lineEnd.graphemeIndex,
        });
        lineWidth += lineWidthContribution;
        remainingWidth = Math.max(0, safeWidth - lineWidth);
        if (lineEnd.segmentIndex === item.endSegmentIndex &&
            lineEnd.graphemeIndex === item.endGraphemeIndex) {
            itemIndex++;
            cursor.segmentIndex = 0;
            cursor.graphemeIndex = 0;
            continue;
        }
        cursor.segmentIndex = lineEnd.segmentIndex;
        cursor.graphemeIndex = lineEnd.graphemeIndex;
        break;
    }
    if (lineWidth === 0)
        return null;
    cursor.itemIndex = itemIndex;
    return lineWidth;
}
export function layoutNextRichInlineLineRange(prepared, maxWidth, start = RICH_INLINE_START_CURSOR) {
    const flow = getInternalPreparedRichInline(prepared);
    const end = {
        itemIndex: start.itemIndex,
        segmentIndex: start.segmentIndex,
        graphemeIndex: start.graphemeIndex,
    };
    const fragments = [];
    const width = stepRichInlineLine(flow, maxWidth, end, (item, gapBefore, occupiedWidth, fragmentStart, fragmentEnd) => {
        fragments.push({
            itemIndex: item.sourceItemIndex,
            gapBefore,
            occupiedWidth,
            start: fragmentStart,
            end: fragmentEnd,
        });
    });
    if (width === null)
        return null;
    return {
        fragments,
        width,
        end,
    };
}
function materializeFragmentText(item, fragment) {
    return buildLineTextFromRange(item.prepared, getLineTextCache(item.prepared), fragment.start.segmentIndex, fragment.start.graphemeIndex, fragment.end.segmentIndex, fragment.end.graphemeIndex);
}
// Bridge from cheap range walking to full fragment text. Lets callers do
// shrinkwrap/virtualization/probing work first, then only pay for text on the
// lines they actually render.
export function materializeRichInlineLineRange(prepared, line) {
    const flow = getInternalPreparedRichInline(prepared);
    const fragments = [];
    for (let i = 0; i < line.fragments.length; i++) {
        const fragment = line.fragments[i];
        const item = flow.itemsBySourceItemIndex[fragment.itemIndex];
        if (item === undefined)
            throw new Error('Missing rich-text inline item for fragment');
        fragments.push({
            itemIndex: fragment.itemIndex,
            text: materializeFragmentText(item, fragment),
            gapBefore: fragment.gapBefore,
            occupiedWidth: fragment.occupiedWidth,
            start: fragment.start,
            end: fragment.end,
        });
    }
    return {
        fragments,
        width: line.width,
        end: line.end,
    };
}
export function walkRichInlineLineRanges(prepared, maxWidth, onLine) {
    let lineCount = 0;
    let cursor = RICH_INLINE_START_CURSOR;
    while (true) {
        const line = layoutNextRichInlineLineRange(prepared, maxWidth, cursor);
        if (line === null)
            return lineCount;
        onLine(line);
        lineCount++;
        cursor = line.end;
    }
}
export function measureRichInlineStats(prepared, maxWidth) {
    const flow = getInternalPreparedRichInline(prepared);
    let lineCount = 0;
    let maxLineWidth = 0;
    const cursor = {
        itemIndex: 0,
        segmentIndex: 0,
        graphemeIndex: 0,
    };
    while (true) {
        const lineWidth = stepRichInlineLine(flow, maxWidth, cursor);
        if (lineWidth === null) {
            return {
                lineCount,
                maxLineWidth,
            };
        }
        lineCount++;
        if (lineWidth > maxLineWidth)
            maxLineWidth = lineWidth;
    }
}
