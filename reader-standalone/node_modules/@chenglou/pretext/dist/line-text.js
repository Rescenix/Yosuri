let sharedGraphemeSegmenter = null;
let sharedLineTextCaches = new WeakMap();
function getSharedGraphemeSegmenter() {
    if (sharedGraphemeSegmenter === null) {
        sharedGraphemeSegmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' });
    }
    return sharedGraphemeSegmenter;
}
function getSegmentGraphemes(segmentIndex, segments, cache) {
    let graphemes = cache.get(segmentIndex);
    if (graphemes !== undefined)
        return graphemes;
    graphemes = [];
    const graphemeSegmenter = getSharedGraphemeSegmenter();
    for (const gs of graphemeSegmenter.segment(segments[segmentIndex])) {
        graphemes.push(gs.segment);
    }
    cache.set(segmentIndex, graphemes);
    return graphemes;
}
function lineHasDiscretionaryHyphen(kinds, startSegmentIndex, endSegmentIndex) {
    return (endSegmentIndex > startSegmentIndex &&
        kinds[endSegmentIndex - 1] === 'soft-hyphen');
}
function appendSegmentGraphemeRange(text, graphemes, startGraphemeIndex, endGraphemeIndex) {
    for (let i = startGraphemeIndex; i < endGraphemeIndex; i++) {
        text += graphemes[i];
    }
    return text;
}
export function getLineTextCache(prepared) {
    let cache = sharedLineTextCaches.get(prepared);
    if (cache !== undefined)
        return cache;
    cache = new Map();
    sharedLineTextCaches.set(prepared, cache);
    return cache;
}
export function buildLineTextFromRange(prepared, cache, startSegmentIndex, startGraphemeIndex, endSegmentIndex, endGraphemeIndex) {
    let text = '';
    const endsWithDiscretionaryHyphen = lineHasDiscretionaryHyphen(prepared.kinds, startSegmentIndex, endSegmentIndex);
    for (let i = startSegmentIndex; i < endSegmentIndex; i++) {
        if (prepared.kinds[i] === 'soft-hyphen' || prepared.kinds[i] === 'hard-break')
            continue;
        if (i === startSegmentIndex && startGraphemeIndex > 0) {
            const graphemes = getSegmentGraphemes(i, prepared.segments, cache);
            text = appendSegmentGraphemeRange(text, graphemes, startGraphemeIndex, graphemes.length);
        }
        else {
            text += prepared.segments[i];
        }
    }
    if (endGraphemeIndex > 0) {
        if (endsWithDiscretionaryHyphen)
            text += '-';
        const graphemes = getSegmentGraphemes(endSegmentIndex, prepared.segments, cache);
        text = appendSegmentGraphemeRange(text, graphemes, startSegmentIndex === endSegmentIndex ? startGraphemeIndex : 0, endGraphemeIndex);
    }
    else if (endsWithDiscretionaryHyphen) {
        text += '-';
    }
    return text;
}
export function clearLineTextCaches() {
    sharedGraphemeSegmenter = null;
    sharedLineTextCaches = new WeakMap();
}
