(function (global, factory) {
	typeof exports === 'object' && typeof module !== 'undefined' ? factory(exports, require('vue')) :
	typeof define === 'function' && define.amd ? define(['exports', 'vue'], factory) :
	(global = typeof globalThis !== 'undefined' ? globalThis : global || self, factory(global.Icon = {}, global.Vue));
})(this, (function (exports, vue) { 'use strict';

	/**
	* Expression to test part of icon name.
	*
	* Used when loading icons from Iconify API due to project naming convension.
	* Ignored when using custom icon sets - convension does not apply.
	*/
	const matchIconName = /^[a-z0-9]+(-[a-z0-9]+)*$/;
	/**
	* Convert string icon name to IconifyIconName object.
	*/
	const stringToIcon = (value, validate, allowSimpleName, provider = "") => {
		const colonSeparated = value.split(":");
		if (value.slice(0, 1) === "@") {
			if (colonSeparated.length < 2 || colonSeparated.length > 3) return null;
			provider = colonSeparated.shift().slice(1);
		}
		if (colonSeparated.length > 3 || !colonSeparated.length) return null;
		if (colonSeparated.length > 1) {
			const name = colonSeparated.pop();
			const prefix = colonSeparated.pop();
			const result = {
				provider: colonSeparated.length > 0 ? colonSeparated[0] : provider,
				prefix,
				name
			};
			return validate && !validateIconName(result) ? null : result;
		}
		const name = colonSeparated[0];
		const dashSeparated = name.split("-");
		if (dashSeparated.length > 1) {
			const result = {
				provider,
				prefix: dashSeparated.shift(),
				name: dashSeparated.join("-")
			};
			return validate && !validateIconName(result) ? null : result;
		}
		if (allowSimpleName && provider === "") {
			const result = {
				provider,
				prefix: "",
				name
			};
			return validate && !validateIconName(result, allowSimpleName) ? null : result;
		}
		return null;
	};
	/**
	* Check if icon is valid.
	*
	* This function is not part of stringToIcon because validation is not needed for most code.
	*/
	const validateIconName = (icon, allowSimpleName) => {
		if (!icon) return false;
		return !!((allowSimpleName && icon.prefix === "" || !!icon.prefix) && !!icon.name);
	};

	/**
	* Resolve icon set icons
	*
	* Returns parent icon for each icon
	*/
	function getIconsTree(data, names) {
		const icons = data.icons;
		const aliases = data.aliases || Object.create(null);
		const resolved = Object.create(null);
		function resolve(name) {
			if (icons[name]) return resolved[name] = [];
			if (!(name in resolved)) {
				resolved[name] = null;
				const parent = aliases[name] && aliases[name].parent;
				const value = parent && resolve(parent);
				if (value) resolved[name] = [parent].concat(value);
			}
			return resolved[name];
		}
		(Object.keys(icons).concat(Object.keys(aliases))).forEach(resolve);
		return resolved;
	}

	/** Default values for dimensions */
	const defaultIconDimensions = Object.freeze({
		left: 0,
		top: 0,
		width: 16,
		height: 16
	});
	/** Default values for transformations */
	const defaultIconTransformations = Object.freeze({
		rotate: 0,
		vFlip: false,
		hFlip: false
	});
	/** Default values for all optional IconifyIcon properties */
	const defaultIconProps = Object.freeze({
		...defaultIconDimensions,
		...defaultIconTransformations
	});
	/** Default values for all properties used in ExtendedIconifyIcon */
	const defaultExtendedIconProps = Object.freeze({
		...defaultIconProps,
		body: "",
		hidden: false
	});

	/**
	* Merge transformations
	*/
	function mergeIconTransformations(obj1, obj2) {
		const result = {};
		if (!obj1.hFlip !== !obj2.hFlip) result.hFlip = true;
		if (!obj1.vFlip !== !obj2.vFlip) result.vFlip = true;
		const rotate = ((obj1.rotate || 0) + (obj2.rotate || 0)) % 4;
		if (rotate) result.rotate = rotate;
		return result;
	}

	/**
	* Merge icon and alias
	*
	* Can also be used to merge default values and icon
	*/
	function mergeIconData(parent, child) {
		const result = mergeIconTransformations(parent, child);
		for (const key in defaultExtendedIconProps) if (key in defaultIconTransformations) {
			if (key in parent && !(key in result)) result[key] = defaultIconTransformations[key];
		} else if (key in child) result[key] = child[key];
		else if (key in parent) result[key] = parent[key];
		return result;
	}

	/**
	* Get icon data, using prepared aliases tree
	*/
	function internalGetIconData(data, name, tree) {
		const icons = data.icons;
		const aliases = data.aliases || Object.create(null);
		let currentProps = {};
		function parse(name) {
			currentProps = mergeIconData(icons[name] || aliases[name], currentProps);
		}
		parse(name);
		tree.forEach(parse);
		return mergeIconData(data, currentProps);
	}

	/**
	* Extract icons from an icon set
	*
	* Returns list of icons that were found in icon set
	*/
	function parseIconSet(data, callback) {
		const names = [];
		if (typeof data !== "object" || typeof data.icons !== "object") return names;
		if (data.not_found instanceof Array) data.not_found.forEach((name) => {
			callback(name, null);
			names.push(name);
		});
		const tree = getIconsTree(data);
		for (const name in tree) {
			const item = tree[name];
			if (item) {
				callback(name, internalGetIconData(data, name, item));
				names.push(name);
			}
		}
		return names;
	}

	/**
	* Optional properties
	*/
	const optionalPropertyDefaults = {
		provider: "",
		aliases: {},
		not_found: {},
		...defaultIconDimensions
	};
	/**
	* Check props
	*/
	function checkOptionalProps(item, defaults) {
		for (const prop in defaults) if (prop in item && typeof item[prop] !== typeof defaults[prop]) return false;
		return true;
	}
	/**
	* Validate icon set, return it as IconifyJSON on success, null on failure
	*
	* Unlike validateIconSet(), this function is very basic.
	* It does not throw exceptions, it does not check metadata, it does not fix stuff.
	*/
	function quicklyValidateIconSet(obj) {
		if (typeof obj !== "object" || obj === null) return null;
		const data = obj;
		if (typeof data.prefix !== "string" || !obj.icons || typeof obj.icons !== "object") return null;
		if (!checkOptionalProps(obj, optionalPropertyDefaults)) return null;
		const icons = data.icons;
		for (const name in icons) {
			const icon = icons[name];
			if (!name || typeof icon.body !== "string" || !checkOptionalProps(icon, defaultExtendedIconProps)) return null;
		}
		const aliases = data.aliases || Object.create(null);
		for (const name in aliases) {
			const icon = aliases[name];
			const parent = icon.parent;
			if (!name || typeof parent !== "string" || !icons[parent] && !aliases[parent] || !checkOptionalProps(icon, defaultExtendedIconProps)) return null;
		}
		return data;
	}

	/**
	* Storage by provider and prefix
	*/
	const dataStorage = Object.create(null);
	/**
	* Create new storage
	*/
	function newStorage(provider, prefix) {
		return {
			provider,
			prefix,
			icons: Object.create(null),
			missing: /* @__PURE__ */ new Set()
		};
	}
	/**
	* Get storage for provider and prefix
	*/
	function getStorage(provider, prefix) {
		const providerStorage = dataStorage[provider] || (dataStorage[provider] = Object.create(null));
		return providerStorage[prefix] || (providerStorage[prefix] = newStorage(provider, prefix));
	}
	/**
	* Add icon set to storage
	*
	* Returns array of added icons
	*/
	function addIconSet(storage, data) {
		if (!quicklyValidateIconSet(data)) return [];
		return parseIconSet(data, (name, icon) => {
			if (icon) storage.icons[name] = icon;
			else storage.missing.add(name);
		});
	}
	/**
	* Add icon to storage
	*/
	function addIconToStorage(storage, name, icon) {
		try {
			if (typeof icon.body === "string") {
				storage.icons[name] = { ...icon };
				return true;
			}
		} catch (err) {}
		return false;
	}
	/**
	* List available icons
	*/
	function listIcons(provider, prefix) {
		let allIcons = [];
		(typeof provider === "string" ? [provider] : Object.keys(dataStorage)).forEach((provider) => {
			(typeof provider === "string" && typeof prefix === "string" ? [prefix] : Object.keys(dataStorage[provider] || {})).forEach((prefix) => {
				const storage = getStorage(provider, prefix);
				allIcons = allIcons.concat(Object.keys(storage.icons).map((name) => (provider !== "" ? "@" + provider + ":" : "") + prefix + ":" + name));
			});
		});
		return allIcons;
	}

	/**
	* Allow storing icons without provider or prefix, making it possible to store icons like "home"
	*/
	let simpleNames = false;
	function allowSimpleNames(allow) {
		if (typeof allow === "boolean") simpleNames = allow;
		return simpleNames;
	}
	/**
	* Get icon data
	*
	* Returns:
	* - IconifyIcon on success, object directly from storage so don't modify it
	* - null if icon is marked as missing (returned in `not_found` property from API, so don't bother sending API requests)
	* - undefined if icon is missing in storage
	*/
	function getIconData(name) {
		const icon = typeof name === "string" ? stringToIcon(name, true, simpleNames) : name;
		if (icon) {
			const storage = getStorage(icon.provider, icon.prefix);
			const iconName = icon.name;
			return storage.icons[iconName] || (storage.missing.has(iconName) ? null : void 0);
		}
	}
	/**
	* Add one icon
	*/
	function addIcon(name, data) {
		const icon = stringToIcon(name, true, simpleNames);
		if (!icon) return false;
		const storage = getStorage(icon.provider, icon.prefix);
		if (data) return addIconToStorage(storage, icon.name, data);
		else {
			storage.missing.add(icon.name);
			return true;
		}
	}
	/**
	* Add icon set
	*/
	function addCollection(data, provider) {
		if (typeof data !== "object") return false;
		if (typeof provider !== "string") provider = data.provider || "";
		if (simpleNames && !provider && !data.prefix) {
			let added = false;
			if (quicklyValidateIconSet(data)) {
				data.prefix = "";
				parseIconSet(data, (name, icon) => {
					if (addIcon(name, icon)) added = true;
				});
			}
			return added;
		}
		const prefix = data.prefix;
		if (!validateIconName({
			prefix,
			name: "a"
		})) return false;
		return !!addIconSet(getStorage(provider, prefix), data);
	}
	/**
	* Check if icon data is available
	*/
	function iconLoaded(name) {
		return !!getIconData(name);
	}
	/**
	* Get full icon
	*/
	function getIcon(name) {
		const result = getIconData(name);
		return result ? {
			...defaultIconProps,
			...result
		} : result;
	}

	/**
	* Default icon customisations values
	*/
	const defaultIconSizeCustomisations = Object.freeze({
		width: null,
		height: null
	});
	const defaultIconCustomisations = Object.freeze({
		...defaultIconSizeCustomisations,
		...defaultIconTransformations
	});

	/**
	* Regular expressions for calculating dimensions
	*/
	const unitsSplit = /(-?[0-9.]*[0-9]+[0-9.]*)/g;
	const unitsTest = /^-?[0-9.]*[0-9]+[0-9.]*$/g;
	function calculateSize(size, ratio, precision) {
		if (ratio === 1) return size;
		precision = precision || 100;
		if (typeof size === "number") return Math.ceil(size * ratio * precision) / precision;
		if (typeof size !== "string") return size;
		const oldParts = size.split(unitsSplit);
		if (oldParts === null || !oldParts.length) return size;
		const newParts = [];
		let code = oldParts.shift();
		let isNumber = unitsTest.test(code);
		while (true) {
			if (isNumber) {
				const num = parseFloat(code);
				if (isNaN(num)) newParts.push(code);
				else newParts.push(Math.ceil(num * ratio * precision) / precision);
			} else newParts.push(code);
			code = oldParts.shift();
			if (code === void 0) return newParts.join("");
			isNumber = !isNumber;
		}
	}

	function splitSVGDefs(content, tag = "defs") {
		let defs = "";
		const index = content.indexOf("<" + tag);
		while (index >= 0) {
			const start = content.indexOf(">", index);
			const end = content.indexOf("</" + tag);
			if (start === -1 || end === -1) break;
			const endEnd = content.indexOf(">", end);
			if (endEnd === -1) break;
			defs += content.slice(start + 1, end).trim();
			content = content.slice(0, index).trim() + content.slice(endEnd + 1);
		}
		return {
			defs,
			content
		};
	}
	/**
	* Merge defs and content
	*/
	function mergeDefsAndContent(defs, content) {
		return defs ? "<defs>" + defs + "</defs>" + content : content;
	}
	/**
	* Wrap SVG content, without wrapping definitions
	*/
	function wrapSVGContent(body, start, end) {
		const split = splitSVGDefs(body);
		return mergeDefsAndContent(split.defs, start + split.content + end);
	}

	/**
	* Check if value should be unset. Allows multiple keywords
	*/
	const isUnsetKeyword = (value) => value === "unset" || value === "undefined" || value === "none";
	/**
	* Get SVG attributes and content from icon + customisations
	*
	* Does not generate style to make it compatible with frameworks that use objects for style, such as React.
	* Instead, it generates 'inline' value. If true, rendering engine should add verticalAlign: -0.125em to icon.
	*
	* Customisations should be normalised by platform specific parser.
	* Result should be converted to <svg> by platform specific parser.
	* Use replaceIDs to generate unique IDs for body.
	*/
	function iconToSVG(icon, customisations) {
		const fullIcon = {
			...defaultIconProps,
			...icon
		};
		const fullCustomisations = {
			...defaultIconCustomisations,
			...customisations
		};
		const box = {
			left: fullIcon.left,
			top: fullIcon.top,
			width: fullIcon.width,
			height: fullIcon.height
		};
		let body = fullIcon.body;
		[fullIcon, fullCustomisations].forEach((props) => {
			const transformations = [];
			const hFlip = props.hFlip;
			const vFlip = props.vFlip;
			let rotation = props.rotate;
			if (hFlip) if (vFlip) rotation += 2;
			else {
				transformations.push("translate(" + (box.width + box.left).toString() + " " + (0 - box.top).toString() + ")");
				transformations.push("scale(-1 1)");
				box.top = box.left = 0;
			}
			else if (vFlip) {
				transformations.push("translate(" + (0 - box.left).toString() + " " + (box.height + box.top).toString() + ")");
				transformations.push("scale(1 -1)");
				box.top = box.left = 0;
			}
			let tempValue;
			if (rotation < 0) rotation -= Math.floor(rotation / 4) * 4;
			rotation = rotation % 4;
			switch (rotation) {
				case 1:
					tempValue = box.height / 2 + box.top;
					transformations.unshift("rotate(90 " + tempValue.toString() + " " + tempValue.toString() + ")");
					break;
				case 2:
					transformations.unshift("rotate(180 " + (box.width / 2 + box.left).toString() + " " + (box.height / 2 + box.top).toString() + ")");
					break;
				case 3:
					tempValue = box.width / 2 + box.left;
					transformations.unshift("rotate(-90 " + tempValue.toString() + " " + tempValue.toString() + ")");
					break;
			}
			if (rotation % 2 === 1) {
				if (box.left !== box.top) {
					tempValue = box.left;
					box.left = box.top;
					box.top = tempValue;
				}
				if (box.width !== box.height) {
					tempValue = box.width;
					box.width = box.height;
					box.height = tempValue;
				}
			}
			if (transformations.length) body = wrapSVGContent(body, "<g transform=\"" + transformations.join(" ") + "\">", "</g>");
		});
		const customisationsWidth = fullCustomisations.width;
		const customisationsHeight = fullCustomisations.height;
		const boxWidth = box.width;
		const boxHeight = box.height;
		let width;
		let height;
		if (customisationsWidth === null) {
			height = customisationsHeight === null ? "1em" : customisationsHeight === "auto" ? boxHeight : customisationsHeight;
			width = calculateSize(height, boxWidth / boxHeight);
		} else {
			width = customisationsWidth === "auto" ? boxWidth : customisationsWidth;
			height = customisationsHeight === null ? calculateSize(width, boxHeight / boxWidth) : customisationsHeight === "auto" ? boxHeight : customisationsHeight;
		}
		const attributes = {};
		const setAttr = (prop, value) => {
			if (!isUnsetKeyword(value)) attributes[prop] = value.toString();
		};
		setAttr("width", width);
		setAttr("height", height);
		const viewBox = [
			box.left,
			box.top,
			boxWidth,
			boxHeight
		];
		attributes.viewBox = viewBox.join(" ");
		return {
			attributes,
			viewBox,
			body
		};
	}

	/**
	* Regular expression for finding ids
	*/
	const regex = /\sid="(\S+)"/g;
	/**
	* Counters
	*/
	const counters = /* @__PURE__ */ new Map();
	/**
	* Get unique new ID
	*/
	function nextID(id) {
		id = id.replace(/[0-9]+$/, "") || "a";
		const count = counters.get(id) || 0;
		counters.set(id, count + 1);
		return count ? `${id}${count}` : id;
	}
	/**
	* Replace IDs in SVG output with unique IDs
	*/
	function replaceIDs(body) {
		const ids = [];
		let match;
		while (match = regex.exec(body)) ids.push(match[1]);
		if (!ids.length) return body;
		const suffix = "suffix" + (Math.random() * 16777216 | Date.now()).toString(16);
		ids.forEach((id) => {
			const newID = nextID(id);
			const escapedID = id.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
			body = body.replace(new RegExp("([#;\"])(" + escapedID + ")([\")]|\\.[a-z])", "g"), "$1" + newID + suffix + "$3");
		});
		body = body.replace(new RegExp(suffix, "g"), "");
		return body;
	}
	/**
	* Clear ID cache
	*/
	function clearIDCache() {
		counters.clear();
	}

	/**
	* Local storate types and entries
	*/
	const storage = Object.create(null);
	/**
	* Set API module
	*/
	function setAPIModule(provider, item) {
		storage[provider] = item;
	}
	/**
	* Get API module
	*/
	function getAPIModule(provider) {
		return storage[provider] || storage[""];
	}

	/**
	* Create full API configuration from partial data
	*/
	function createAPIConfig(source) {
		let resources;
		if (typeof source.resources === "string") resources = [source.resources];
		else {
			resources = source.resources;
			if (!(resources instanceof Array) || !resources.length) return null;
		}
		return {
			resources,
			path: source.path || "/",
			maxURL: source.maxURL || 500,
			rotate: source.rotate || 750,
			timeout: source.timeout || 5e3,
			random: source.random === true,
			index: source.index || 0,
			dataAfterTimeout: source.dataAfterTimeout !== false
		};
	}
	/**
	* Local storage
	*/
	const configStorage = Object.create(null);
	/**
	* Redundancy for API servers.
	*
	* API should have very high uptime because of implemented redundancy at server level, but
	* sometimes bad things happen. On internet 100% uptime is not possible.
	*
	* There could be routing problems. Server might go down for whatever reason, but it takes
	* few minutes to detect that downtime, so during those few minutes API might not be accessible.
	*
	* This script has some redundancy to mitigate possible network issues.
	*
	* If one host cannot be reached in 'rotate' (750 by default) ms, script will try to retrieve
	* data from different host. Hosts have different configurations, pointing to different
	* API servers hosted at different providers.
	*/
	const fallBackAPISources = ["https://api.simplesvg.com", "https://api.unisvg.com"];
	const fallBackAPI = [];
	while (fallBackAPISources.length > 0) if (fallBackAPISources.length === 1) fallBackAPI.push(fallBackAPISources.shift());
	else if (Math.random() > .5) fallBackAPI.push(fallBackAPISources.shift());
	else fallBackAPI.push(fallBackAPISources.pop());
	configStorage[""] = createAPIConfig({ resources: ["https://api.iconify.design"].concat(fallBackAPI) });
	/**
	* Add custom config for provider
	*/
	function addAPIProvider(provider, customConfig) {
		const config = createAPIConfig(customConfig);
		if (config === null) return false;
		configStorage[provider] = config;
		return true;
	}
	/**
	* Get API configuration
	*/
	function getAPIConfig(provider) {
		return configStorage[provider];
	}
	/**
	* List API providers
	*/
	function listAPIProviders() {
		return Object.keys(configStorage);
	}

	const detectFetch = () => {
		let callback;
		try {
			callback = fetch;
			if (typeof callback === "function") return callback;
		} catch (err) {}
	};
	/**
	* Fetch function
	*/
	let fetchModule = detectFetch();
	/**
	* Set custom fetch() function
	*/
	function setFetch(fetch) {
		fetchModule = fetch;
	}
	/**
	* Get fetch() function. Used by Icon Finder Core
	*/
	function getFetch() {
		return fetchModule;
	}
	/**
	* Calculate maximum icons list length for prefix
	*/
	function calculateMaxLength(provider, prefix) {
		const config = getAPIConfig(provider);
		if (!config) return 0;
		let result;
		if (!config.maxURL) result = 0;
		else {
			let maxHostLength = 0;
			config.resources.forEach((item) => {
				maxHostLength = Math.max(maxHostLength, item.length);
			});
			const url = prefix + ".json?icons=";
			result = config.maxURL - maxHostLength - config.path.length - url.length;
		}
		return result;
	}
	/**
	* Should query be aborted, based on last HTTP status
	*/
	function shouldAbort(status) {
		return status === 404;
	}
	/**
	* Prepare params
	*/
	const prepare = (provider, prefix, icons) => {
		const results = [];
		const maxLength = calculateMaxLength(provider, prefix);
		const type = "icons";
		let item = {
			type,
			provider,
			prefix,
			icons: []
		};
		let length = 0;
		icons.forEach((name, index) => {
			length += name.length + 1;
			if (length >= maxLength && index > 0) {
				results.push(item);
				item = {
					type,
					provider,
					prefix,
					icons: []
				};
				length = name.length;
			}
			item.icons.push(name);
		});
		results.push(item);
		return results;
	};
	/**
	* Get path
	*/
	function getPath(provider) {
		if (typeof provider === "string") {
			const config = getAPIConfig(provider);
			if (config) return config.path;
		}
		return "/";
	}
	/**
	* Load icons
	*/
	const send = (host, params, callback) => {
		if (!fetchModule) {
			callback("abort", 424);
			return;
		}
		let path = getPath(params.provider);
		switch (params.type) {
			case "icons": {
				const prefix = params.prefix;
				const iconsList = params.icons.join(",");
				const urlParams = new URLSearchParams({ icons: iconsList });
				path += prefix + ".json?" + urlParams.toString();
				break;
			}
			case "custom": {
				const uri = params.uri;
				path += uri.slice(0, 1) === "/" ? uri.slice(1) : uri;
				break;
			}
			default:
				callback("abort", 400);
				return;
		}
		let defaultError = 503;
		fetchModule(host + path).then((response) => {
			const status = response.status;
			if (status !== 200) {
				setTimeout(() => {
					callback(shouldAbort(status) ? "abort" : "next", status);
				});
				return;
			}
			defaultError = 501;
			return response.json();
		}).then((data) => {
			if (typeof data !== "object" || data === null) {
				setTimeout(() => {
					if (data === 404) callback("abort", data);
					else callback("next", defaultError);
				});
				return;
			}
			setTimeout(() => {
				callback("success", data);
			});
		}).catch(() => {
			callback("next", defaultError);
		});
	};
	/**
	* Export module
	*/
	const fetchAPIModule = {
		prepare,
		send
	};

	/**
	* Remove callback
	*/
	function removeCallback(storages, id) {
		storages.forEach((storage) => {
			const items = storage.loaderCallbacks;
			if (items) storage.loaderCallbacks = items.filter((row) => row.id !== id);
		});
	}
	/**
	* Update all callbacks for provider and prefix
	*/
	function updateCallbacks(storage) {
		if (!storage.pendingCallbacksFlag) {
			storage.pendingCallbacksFlag = true;
			setTimeout(() => {
				storage.pendingCallbacksFlag = false;
				const items = storage.loaderCallbacks ? storage.loaderCallbacks.slice(0) : [];
				if (!items.length) return;
				let hasPending = false;
				const provider = storage.provider;
				const prefix = storage.prefix;
				items.forEach((item) => {
					const icons = item.icons;
					const oldLength = icons.pending.length;
					icons.pending = icons.pending.filter((icon) => {
						if (icon.prefix !== prefix) return true;
						const name = icon.name;
						if (storage.icons[name]) icons.loaded.push({
							provider,
							prefix,
							name
						});
						else if (storage.missing.has(name)) icons.missing.push({
							provider,
							prefix,
							name
						});
						else {
							hasPending = true;
							return true;
						}
						return false;
					});
					if (icons.pending.length !== oldLength) {
						if (!hasPending) removeCallback([storage], item.id);
						item.callback(icons.loaded.slice(0), icons.missing.slice(0), icons.pending.slice(0), item.abort);
					}
				});
			});
		}
	}
	/**
	* Unique id counter for callbacks
	*/
	let idCounter = 0;
	/**
	* Add callback
	*/
	function storeCallback(callback, icons, pendingSources) {
		const id = idCounter++;
		const abort = removeCallback.bind(null, pendingSources, id);
		if (!icons.pending.length) return abort;
		const item = {
			id,
			icons,
			callback,
			abort
		};
		pendingSources.forEach((storage) => {
			(storage.loaderCallbacks || (storage.loaderCallbacks = [])).push(item);
		});
		return abort;
	}

	/**
	* Check if icons have been loaded
	*/
	function sortIcons(icons) {
		const result = {
			loaded: [],
			missing: [],
			pending: []
		};
		const storage = Object.create(null);
		icons.sort((a, b) => {
			if (a.provider !== b.provider) return a.provider.localeCompare(b.provider);
			if (a.prefix !== b.prefix) return a.prefix.localeCompare(b.prefix);
			return a.name.localeCompare(b.name);
		});
		let lastIcon = {
			provider: "",
			prefix: "",
			name: ""
		};
		icons.forEach((icon) => {
			if (lastIcon.name === icon.name && lastIcon.prefix === icon.prefix && lastIcon.provider === icon.provider) return;
			lastIcon = icon;
			const provider = icon.provider;
			const prefix = icon.prefix;
			const name = icon.name;
			const providerStorage = storage[provider] || (storage[provider] = Object.create(null));
			const localStorage = providerStorage[prefix] || (providerStorage[prefix] = getStorage(provider, prefix));
			let list;
			if (name in localStorage.icons) list = result.loaded;
			else if (prefix === "" || localStorage.missing.has(name)) list = result.missing;
			else list = result.pending;
			const item = {
				provider,
				prefix,
				name
			};
			list.push(item);
		});
		return result;
	}

	/**
	* Convert icons list from string/icon mix to icons and validate them
	*/
	function listToIcons(list, validate = true, simpleNames = false) {
		const result = [];
		list.forEach((item) => {
			const icon = typeof item === "string" ? stringToIcon(item, validate, simpleNames) : item;
			if (icon) result.push(icon);
		});
		return result;
	}

	/**
	* Default RedundancyConfig for API calls
	*/
	const defaultConfig = {
		resources: [],
		index: 0,
		timeout: 2e3,
		rotate: 750,
		random: false,
		dataAfterTimeout: false
	};
	/**
	* Send query
	*/
	function sendQuery(config, payload, query, done) {
		const resourcesCount = config.resources.length;
		const startIndex = config.random ? Math.floor(Math.random() * resourcesCount) : config.index;
		let resources;
		if (config.random) {
			let list = config.resources.slice(0);
			resources = [];
			while (list.length > 1) {
				const nextIndex = Math.floor(Math.random() * list.length);
				resources.push(list[nextIndex]);
				list = list.slice(0, nextIndex).concat(list.slice(nextIndex + 1));
			}
			resources = resources.concat(list);
		} else resources = config.resources.slice(startIndex).concat(config.resources.slice(0, startIndex));
		const startTime = Date.now();
		let status = "pending";
		let queriesSent = 0;
		let lastError;
		let timer = null;
		let queue = [];
		let doneCallbacks = [];
		if (typeof done === "function") doneCallbacks.push(done);
		/**
		* Reset timer
		*/
		function resetTimer() {
			if (timer) {
				clearTimeout(timer);
				timer = null;
			}
		}
		/**
		* Abort everything
		*/
		function abort() {
			if (status === "pending") status = "aborted";
			resetTimer();
			queue.forEach((item) => {
				if (item.status === "pending") item.status = "aborted";
			});
			queue = [];
		}
		/**
		* Add / replace callback to call when execution is complete.
		* This can be used to abort pending query implementations when query is complete or aborted.
		*/
		function subscribe(callback, overwrite) {
			if (overwrite) doneCallbacks = [];
			if (typeof callback === "function") doneCallbacks.push(callback);
		}
		/**
		* Get query status
		*/
		function getQueryStatus() {
			return {
				startTime,
				payload,
				status,
				queriesSent,
				queriesPending: queue.length,
				subscribe,
				abort
			};
		}
		/**
		* Fail query
		*/
		function failQuery() {
			status = "failed";
			doneCallbacks.forEach((callback) => {
				callback(void 0, lastError);
			});
		}
		/**
		* Clear queue
		*/
		function clearQueue() {
			queue.forEach((item) => {
				if (item.status === "pending") item.status = "aborted";
			});
			queue = [];
		}
		/**
		* Got response from module
		*/
		function moduleResponse(item, response, data) {
			const isError = response !== "success";
			queue = queue.filter((queued) => queued !== item);
			switch (status) {
				case "pending": break;
				case "failed":
					if (isError || !config.dataAfterTimeout) return;
					break;
				default: return;
			}
			if (response === "abort") {
				lastError = data;
				failQuery();
				return;
			}
			if (isError) {
				lastError = data;
				if (!queue.length) if (!resources.length) failQuery();
				else execNext();
				return;
			}
			resetTimer();
			clearQueue();
			if (!config.random) {
				const index = config.resources.indexOf(item.resource);
				if (index !== -1 && index !== config.index) config.index = index;
			}
			status = "completed";
			doneCallbacks.forEach((callback) => {
				callback(data);
			});
		}
		/**
		* Execute next query
		*/
		function execNext() {
			if (status !== "pending") return;
			resetTimer();
			const resource = resources.shift();
			if (resource === void 0) {
				if (queue.length) {
					timer = setTimeout(() => {
						resetTimer();
						if (status === "pending") {
							clearQueue();
							failQuery();
						}
					}, config.timeout);
					return;
				}
				failQuery();
				return;
			}
			const item = {
				status: "pending",
				resource,
				callback: (status, data) => {
					moduleResponse(item, status, data);
				}
			};
			queue.push(item);
			queriesSent++;
			timer = setTimeout(execNext, config.rotate);
			query(resource, payload, item.callback);
		}
		setTimeout(execNext);
		return getQueryStatus;
	}
	/**
	* Redundancy instance
	*/
	function initRedundancy(cfg) {
		const config = {
			...defaultConfig,
			...cfg
		};
		let queries = [];
		/**
		* Remove aborted and completed queries
		*/
		function cleanup() {
			queries = queries.filter((item) => item().status === "pending");
		}
		/**
		* Send query
		*/
		function query(payload, queryCallback, doneCallback) {
			const query = sendQuery(config, payload, queryCallback, (data, error) => {
				cleanup();
				if (doneCallback) doneCallback(data, error);
			});
			queries.push(query);
			return query;
		}
		/**
		* Find instance
		*/
		function find(callback) {
			return queries.find((value) => {
				return callback(value);
			}) || null;
		}
		return {
			query,
			find,
			setIndex: (index) => {
				config.index = index;
			},
			getIndex: () => config.index,
			cleanup
		};
	}

	function emptyCallback$1() {}
	const redundancyCache = Object.create(null);
	/**
	* Get Redundancy instance for provider
	*/
	function getRedundancyCache(provider) {
		if (!redundancyCache[provider]) {
			const config = getAPIConfig(provider);
			if (!config) return;
			redundancyCache[provider] = {
				config,
				redundancy: initRedundancy(config)
			};
		}
		return redundancyCache[provider];
	}
	/**
	* Send API query
	*/
	function sendAPIQuery(target, query, callback) {
		let redundancy;
		let send;
		if (typeof target === "string") {
			const api = getAPIModule(target);
			if (!api) {
				callback(void 0, 424);
				return emptyCallback$1;
			}
			send = api.send;
			const cached = getRedundancyCache(target);
			if (cached) redundancy = cached.redundancy;
		} else {
			const config = createAPIConfig(target);
			if (config) {
				redundancy = initRedundancy(config);
				const api = getAPIModule(target.resources ? target.resources[0] : "");
				if (api) send = api.send;
			}
		}
		if (!redundancy || !send) {
			callback(void 0, 424);
			return emptyCallback$1;
		}
		return redundancy.query(query, send, callback)().abort;
	}

	function emptyCallback() {}
	/**
	* Function called when new icons have been loaded
	*/
	function loadedNewIcons(storage) {
		if (!storage.iconsLoaderFlag) {
			storage.iconsLoaderFlag = true;
			setTimeout(() => {
				storage.iconsLoaderFlag = false;
				updateCallbacks(storage);
			});
		}
	}
	/**
	* Check icon names for API
	*/
	function checkIconNamesForAPI(icons) {
		const valid = [];
		const invalid = [];
		icons.forEach((name) => {
			(name.match(matchIconName) ? valid : invalid).push(name);
		});
		return {
			valid,
			invalid
		};
	}
	/**
	* Parse loader response
	*/
	function parseLoaderResponse(storage, icons, data) {
		function checkMissing() {
			const pending = storage.pendingIcons;
			icons.forEach((name) => {
				if (pending) pending.delete(name);
				if (!storage.icons[name]) storage.missing.add(name);
			});
		}
		if (data && typeof data === "object") try {
			if (!addIconSet(storage, data).length) {
				checkMissing();
				return;
			}
		} catch (err) {
			console.error(err);
		}
		checkMissing();
		loadedNewIcons(storage);
	}
	/**
	* Handle response that can be async
	*/
	function parsePossiblyAsyncResponse(response, callback) {
		if (response instanceof Promise) response.then((data) => {
			callback(data);
		}).catch(() => {
			callback(null);
		});
		else callback(response);
	}
	/**
	* Load icons
	*/
	function loadNewIcons(storage, icons) {
		if (!storage.iconsToLoad) storage.iconsToLoad = icons;
		else storage.iconsToLoad = storage.iconsToLoad.concat(icons).sort();
		if (!storage.iconsQueueFlag) {
			storage.iconsQueueFlag = true;
			setTimeout(() => {
				storage.iconsQueueFlag = false;
				const { provider, prefix } = storage;
				const icons = storage.iconsToLoad;
				delete storage.iconsToLoad;
				if (!icons || !icons.length) return;
				const customIconLoader = storage.loadIcon;
				if (storage.loadIcons && (icons.length > 1 || !customIconLoader)) {
					parsePossiblyAsyncResponse(storage.loadIcons(icons, prefix, provider), (data) => {
						parseLoaderResponse(storage, icons, data);
					});
					return;
				}
				if (customIconLoader) {
					icons.forEach((name) => {
						parsePossiblyAsyncResponse(customIconLoader(name, prefix, provider), (data) => {
							parseLoaderResponse(storage, [name], data ? {
								prefix,
								icons: { [name]: data }
							} : null);
						});
					});
					return;
				}
				const { valid, invalid } = checkIconNamesForAPI(icons);
				if (invalid.length) parseLoaderResponse(storage, invalid, null);
				if (!valid.length) return;
				const api = prefix.match(matchIconName) ? getAPIModule(provider) : null;
				if (!api) {
					parseLoaderResponse(storage, valid, null);
					return;
				}
				api.prepare(provider, prefix, valid).forEach((item) => {
					sendAPIQuery(provider, item, (data) => {
						parseLoaderResponse(storage, item.icons, data);
					});
				});
			});
		}
	}
	/**
	* Load icons
	*/
	const loadIcons = (icons, callback) => {
		const sortedIcons = sortIcons(listToIcons(icons, true, allowSimpleNames()));
		if (!sortedIcons.pending.length) {
			let callCallback = true;
			if (callback) setTimeout(() => {
				if (callCallback) callback(sortedIcons.loaded, sortedIcons.missing, sortedIcons.pending, emptyCallback);
			});
			return () => {
				callCallback = false;
			};
		}
		const newIcons = Object.create(null);
		const sources = [];
		let lastProvider, lastPrefix;
		sortedIcons.pending.forEach((icon) => {
			const { provider, prefix } = icon;
			if (prefix === lastPrefix && provider === lastProvider) return;
			lastProvider = provider;
			lastPrefix = prefix;
			sources.push(getStorage(provider, prefix));
			const providerNewIcons = newIcons[provider] || (newIcons[provider] = Object.create(null));
			if (!providerNewIcons[prefix]) providerNewIcons[prefix] = [];
		});
		sortedIcons.pending.forEach((icon) => {
			const { provider, prefix, name } = icon;
			const storage = getStorage(provider, prefix);
			const pendingQueue = storage.pendingIcons || (storage.pendingIcons = /* @__PURE__ */ new Set());
			if (!pendingQueue.has(name)) {
				pendingQueue.add(name);
				newIcons[provider][prefix].push(name);
			}
		});
		sources.forEach((storage) => {
			const list = newIcons[storage.provider][storage.prefix];
			if (list.length) loadNewIcons(storage, list);
		});
		return callback ? storeCallback(callback, sortedIcons, sources) : emptyCallback;
	};
	/**
	* Load one icon using Promise
	*/
	const loadIcon = (icon) => {
		return new Promise((fulfill, reject) => {
			const iconObj = typeof icon === "string" ? stringToIcon(icon, true) : icon;
			if (!iconObj) {
				reject(icon);
				return;
			}
			loadIcons([iconObj || icon], (loaded) => {
				if (loaded.length && iconObj) {
					const data = getIconData(iconObj);
					if (data) {
						fulfill({
							...defaultIconProps,
							...data
						});
						return;
					}
				}
				reject(icon);
			});
		});
	};

	/**
	* Set custom loader for multiple icons
	*/
	function setCustomIconsLoader(loader, prefix, provider) {
		getStorage(provider || "", prefix).loadIcons = loader;
	}
	/**
	* Set custom loader for one icon
	*/
	function setCustomIconLoader(loader, prefix, provider) {
		getStorage(provider || "", prefix).loadIcon = loader;
	}

	/**
	* Convert IconifyIconCustomisations to FullIconCustomisations, checking value types
	*/
	function mergeCustomisations(defaults, item) {
		const result = { ...defaults };
		for (const key in item) {
			const value = item[key];
			const valueType = typeof value;
			if (key in defaultIconSizeCustomisations) {
				if (value === null || value && (valueType === "string" || valueType === "number")) result[key] = value;
			} else if (valueType === typeof result[key]) result[key] = key === "rotate" ? value % 4 : value;
		}
		return result;
	}

	const separator = /[\s,]+/;
	/**
	* Apply "flip" string to icon customisations
	*/
	function flipFromString(custom, flip) {
		flip.split(separator).forEach((str) => {
			switch (str.trim()) {
				case "horizontal":
					custom.hFlip = true;
					break;
				case "vertical":
					custom.vFlip = true;
					break;
			}
		});
	}

	/**
	* Get rotation value
	*/
	function rotateFromString(value, defaultValue = 0) {
		const units = value.replace(/^-?[0-9.]*/, "");
		function cleanup(value) {
			while (value < 0) value += 4;
			return value % 4;
		}
		if (units === "") {
			const num = parseInt(value);
			return isNaN(num) ? 0 : cleanup(num);
		} else if (units !== value) {
			let split = 0;
			switch (units) {
				case "%":
					split = 25;
					break;
				case "deg": split = 90;
			}
			if (split) {
				let num = parseFloat(value.slice(0, value.length - units.length));
				if (isNaN(num)) return 0;
				num = num / split;
				return num % 1 === 0 ? cleanup(num) : 0;
			}
		}
		return defaultValue;
	}

	/**
	* Generate <svg>
	*/
	function iconToHTML(body, attributes) {
		let renderAttribsHTML = body.indexOf("xlink:") === -1 ? "" : " xmlns:xlink=\"http://www.w3.org/1999/xlink\"";
		for (const attr in attributes) renderAttribsHTML += " " + attr + "=\"" + attributes[attr] + "\"";
		return "<svg xmlns=\"http://www.w3.org/2000/svg\"" + renderAttribsHTML + ">" + body + "</svg>";
	}

	/**
	* Encode SVG for use in url()
	*
	* Short alternative to encodeURIComponent() that encodes only stuff used in SVG, generating
	* smaller code.
	*/
	function encodeSVGforURL(svg) {
		return svg.replace(/"/g, "'").replace(/%/g, "%25").replace(/#/g, "%23").replace(/</g, "%3C").replace(/>/g, "%3E").replace(/\s+/g, " ");
	}
	/**
	* Generate data: URL from SVG
	*/
	function svgToData(svg) {
		return "data:image/svg+xml," + encodeSVGforURL(svg);
	}
	/**
	* Generate url() from SVG
	*/
	function svgToURL(svg) {
		return "url(\"" + svgToData(svg) + "\")";
	}

	const defaultExtendedIconCustomisations = {
	    ...defaultIconCustomisations,
	    inline: false,
	};

	/**
	 * Default SVG attributes
	 */
	const svgDefaults = {
	    'xmlns': 'http://www.w3.org/2000/svg',
	    'xmlns:xlink': 'http://www.w3.org/1999/xlink',
	    'aria-hidden': true,
	    'role': 'img',
	};
	/**
	 * Style modes
	 */
	const commonProps = {
	    display: 'inline-block',
	};
	const monotoneProps = {
	    backgroundColor: 'currentColor',
	};
	const coloredProps = {
	    backgroundColor: 'transparent',
	};
	// Dynamically add common props to variables above
	const propsToAdd = {
	    Image: 'var(--svg)',
	    Repeat: 'no-repeat',
	    Size: '100% 100%',
	};
	const propsToAddTo = {
	    webkitMask: monotoneProps,
	    mask: monotoneProps,
	    background: coloredProps,
	};
	for (const prefix in propsToAddTo) {
	    const list = propsToAddTo[prefix];
	    for (const prop in propsToAdd) {
	        list[prefix + prop] = propsToAdd[prop];
	    }
	}
	/**
	 * Aliases for customisations.
	 * In Vue 'v-' properties are reserved, so v-flip must be renamed
	 */
	const customisationAliases = {};
	['horizontal', 'vertical'].forEach((prefix) => {
	    const attr = prefix.slice(0, 1) + 'Flip';
	    // vertical-flip
	    customisationAliases[prefix + '-flip'] = attr;
	    // v-flip
	    customisationAliases[prefix.slice(0, 1) + '-flip'] = attr;
	    // verticalFlip
	    customisationAliases[prefix + 'Flip'] = attr;
	});
	/**
	 * Fix size: add 'px' to numbers
	 */
	function fixSize(value) {
	    return value + (value.match(/^[-0-9.]+$/) ? 'px' : '');
	}
	/**
	 * Render icon
	 */
	const render = (
	// Icon must be validated before calling this function
	icon, 
	// Partial properties
	props) => {
	    // Split properties
	    const customisations = mergeCustomisations(defaultExtendedIconCustomisations, props);
	    const componentProps = { ...svgDefaults };
	    // Check mode
	    const mode = props.mode || 'svg';
	    // Copy style
	    const style = {};
	    const propsStyle = props.style;
	    const customStyle = typeof propsStyle === 'object' && !(propsStyle instanceof Array)
	        ? propsStyle
	        : {};
	    // Get element properties
	    for (let key in props) {
	        const value = props[key];
	        if (value === void 0) {
	            continue;
	        }
	        switch (key) {
	            // Properties to ignore
	            case 'icon':
	            case 'style':
	            case 'onLoad':
	            case 'mode':
	            case 'ssr':
	            case 'customise':
	                break;
	            // Boolean attributes
	            case 'inline':
	            case 'hFlip':
	            case 'vFlip':
	                customisations[key] =
	                    value === true || value === 'true' || value === 1;
	                break;
	            // Flip as string: 'horizontal,vertical'
	            case 'flip':
	                if (typeof value === 'string') {
	                    flipFromString(customisations, value);
	                }
	                break;
	            // Color: override style
	            case 'color':
	                style.color = value;
	                break;
	            // Rotation as string
	            case 'rotate':
	                if (typeof value === 'string') {
	                    customisations[key] = rotateFromString(value);
	                }
	                else if (typeof value === 'number') {
	                    customisations[key] = value;
	                }
	                break;
	            // Remove aria-hidden
	            case 'ariaHidden':
	            case 'aria-hidden':
	                // Vue transforms 'aria-hidden' property to 'ariaHidden'
	                if (value !== true && value !== 'true') {
	                    delete componentProps['aria-hidden'];
	                }
	                break;
	            default: {
	                const alias = customisationAliases[key];
	                if (alias) {
	                    // Aliases for boolean customisations
	                    if (value === true || value === 'true' || value === 1) {
	                        customisations[alias] = true;
	                    }
	                }
	                else if (defaultExtendedIconCustomisations[key] === void 0) {
	                    // Copy missing property if it does not exist in customisations
	                    componentProps[key] = value;
	                }
	            }
	        }
	    }
	    // Generate icon
	    const item = iconToSVG(icon, customisations);
	    const renderAttribs = item.attributes;
	    // Inline display
	    if (customisations.inline) {
	        style.verticalAlign = '-0.125em';
	    }
	    if (mode === 'svg') {
	        // Add style
	        componentProps.style = {
	            ...style,
	            ...customStyle,
	        };
	        // Add icon stuff
	        Object.assign(componentProps, renderAttribs);
	        // Add innerHTML and style to props
	        componentProps['innerHTML'] = replaceIDs(item.body);
	        // Render icon
	        return vue.h('svg', componentProps);
	    }
	    // Render <span> with style
	    const { body, width, height } = icon;
	    const useMask = mode === 'mask' ||
	        (mode === 'bg' ? false : body.indexOf('currentColor') !== -1);
	    // Generate SVG
	    const html = iconToHTML(body, {
	        ...renderAttribs,
	        width: width + '',
	        height: height + '',
	    });
	    // Generate style
	    componentProps.style = {
	        ...style,
	        '--svg': svgToURL(html),
	        'width': fixSize(renderAttribs.width),
	        'height': fixSize(renderAttribs.height),
	        ...commonProps,
	        ...(useMask ? monotoneProps : coloredProps),
	        ...customStyle,
	    };
	    return vue.h('span', componentProps);
	};

	/**
	 * Initialise stuff
	 */
	// Enable short names
	allowSimpleNames(true);
	// Set API module
	setAPIModule('', fetchAPIModule);
	/**
	 * Browser stuff
	 */
	if (typeof document !== 'undefined' && typeof window !== 'undefined') {
	    const _window = window;
	    // Load icons from global "IconifyPreload"
	    if (_window.IconifyPreload !== void 0) {
	        const preload = _window.IconifyPreload;
	        const err = 'Invalid IconifyPreload syntax.';
	        if (typeof preload === 'object' && preload !== null) {
	            (preload instanceof Array ? preload : [preload]).forEach((item) => {
	                try {
	                    if (
	                    // Check if item is an object and not null/array
	                    typeof item !== 'object' ||
	                        item === null ||
	                        item instanceof Array ||
	                        // Check for 'icons' and 'prefix'
	                        typeof item.icons !== 'object' ||
	                        typeof item.prefix !== 'string' ||
	                        // Add icon set
	                        !addCollection(item)) {
	                        console.error(err);
	                    }
	                }
	                catch (e) {
	                    console.error(err);
	                }
	            });
	        }
	    }
	    // Set API from global "IconifyProviders"
	    if (_window.IconifyProviders !== void 0) {
	        const providers = _window.IconifyProviders;
	        if (typeof providers === 'object' && providers !== null) {
	            for (let key in providers) {
	                const err = 'IconifyProviders[' + key + '] is invalid.';
	                try {
	                    const value = providers[key];
	                    if (typeof value !== 'object' ||
	                        !value ||
	                        value.resources === void 0) {
	                        continue;
	                    }
	                    if (!addAPIProvider(key, value)) {
	                        console.error(err);
	                    }
	                }
	                catch (e) {
	                    console.error(err);
	                }
	            }
	        }
	    }
	}
	/**
	 * Empty icon data, rendered when icon is not available
	 */
	const emptyIcon = {
	    ...defaultIconProps,
	    body: '',
	};
	/**
	 * Component
	 */
	const Icon = vue.defineComponent((props, { emit }) => {
	    const loader = vue.ref(null);
	    function abortLoading() {
	        if (loader.value) {
	            loader.value.abort?.();
	            loader.value = null;
	        }
	    }
	    // Render state
	    const rendering = vue.ref(!!props.ssr);
	    const lastRenderedIconName = vue.ref('');
	    const iconData = vue.shallowRef(null);
	    // Update icon data
	    function getIcon() {
	        const icon = props.icon;
	        // Icon is an object
	        if (typeof icon === 'object' &&
	            icon !== null &&
	            typeof icon.body === 'string') {
	            lastRenderedIconName.value = '';
	            return {
	                data: icon,
	            };
	        }
	        // Check for valid icon name
	        let iconName;
	        if (typeof icon !== 'string' ||
	            (iconName = stringToIcon(icon, false, true)) === null) {
	            return null;
	        }
	        // Load icon
	        let data = getIconData(iconName);
	        if (!data) {
	            // Icon data is not available
	            const oldState = loader.value;
	            if (!oldState || oldState.name !== icon) {
	                // Icon name does not match old loader state
	                if (data === null) {
	                    // Failed to load
	                    loader.value = {
	                        name: icon,
	                    };
	                }
	                else {
	                    loader.value = {
	                        name: icon,
	                        abort: loadIcons([iconName], updateIconData),
	                    };
	                }
	            }
	            return null;
	        }
	        // Icon data is available
	        abortLoading();
	        if (lastRenderedIconName.value !== icon) {
	            lastRenderedIconName.value = icon;
	            // Emit on next tick because render will be called on next tick
	            vue.nextTick(() => {
	                emit('load', icon);
	            });
	        }
	        // Customise icon
	        const customise = props.customise;
	        if (customise) {
	            // Clone data and customise it
	            data = Object.assign({}, data);
	            const customised = customise(data.body, iconName.name, iconName.prefix, iconName.provider);
	            if (typeof customised === 'string') {
	                data.body = customised;
	            }
	        }
	        // Add classes
	        const classes = ['iconify'];
	        if (iconName.prefix !== '') {
	            classes.push('iconify--' + iconName.prefix);
	        }
	        if (iconName.provider !== '') {
	            classes.push('iconify--' + iconName.provider);
	        }
	        return { data, classes };
	    }
	    function updateIconData() {
	        const icon = getIcon();
	        if (!icon) {
	            iconData.value = null;
	        }
	        else if (icon.data !== iconData.value?.data) {
	            iconData.value = icon;
	        }
	    }
	    // Set icon data
	    if (rendering.value) {
	        updateIconData();
	    }
	    else {
	        vue.onMounted(() => {
	            rendering.value = true;
	            updateIconData();
	        });
	    }
	    vue.watch(() => props.icon, updateIconData);
	    // Abort loading on unmount
	    vue.onUnmounted(abortLoading);
	    // Render function
	    return () => {
	        // Get icon data
	        const icon = iconData.value;
	        if (!icon) {
	            // Icon is not available
	            return render(emptyIcon, props);
	        }
	        // Add classes
	        let newProps = props;
	        if (icon.classes) {
	            newProps = {
	                ...props,
	                class: icon.classes.join(' '),
	            };
	        }
	        // Render icon
	        return render({
	            ...defaultIconProps,
	            ...icon.data,
	        }, newProps);
	    };
	}, {
	    props: [
	        // Icon and render mode
	        'icon',
	        'mode',
	        'ssr',
	        // Layout and style
	        'width',
	        'height',
	        'style',
	        'color',
	        'inline',
	        // Transformations
	        'rotate',
	        'hFlip',
	        'horizontalFlip',
	        'vFlip',
	        'verticalFlip',
	        'flip',
	        // Misc
	        'id',
	        'ariaHidden',
	        'customise',
	        'title',
	    ],
	    emits: ['load'],
	});
	/**
	 * Internal API
	 */
	const _api = {
	    getAPIConfig,
	    setAPIModule,
	    sendAPIQuery,
	    setFetch,
	    getFetch,
	    listAPIProviders,
	};

	exports.Icon = Icon;
	exports._api = _api;
	exports.addAPIProvider = addAPIProvider;
	exports.addCollection = addCollection;
	exports.addIcon = addIcon;
	exports.buildIcon = iconToSVG;
	exports.calculateSize = calculateSize;
	exports.clearIDCache = clearIDCache;
	exports.getIcon = getIcon;
	exports.iconLoaded = iconLoaded;
	exports.listIcons = listIcons;
	exports.loadIcon = loadIcon;
	exports.loadIcons = loadIcons;
	exports.replaceIDs = replaceIDs;
	exports.setCustomIconLoader = setCustomIconLoader;
	exports.setCustomIconsLoader = setCustomIconsLoader;

}));
