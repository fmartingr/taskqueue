// node_modules/@vue/shared/dist/shared.esm-bundler.js
function makeMap(str) {
  const map = /* @__PURE__ */ Object.create(null);
  for (const key of str.split(","))
    map[key] = 1;
  return (val) => (val in map);
}
var EMPTY_OBJ = {};
var EMPTY_ARR = [];
var NOOP = () => {};
var NO = () => false;
var isOn = (key) => key.charCodeAt(0) === 111 && key.charCodeAt(1) === 110 && (key.charCodeAt(2) > 122 || key.charCodeAt(2) < 97);
var isModelListener = (key) => key.startsWith("onUpdate:");
var extend = Object.assign;
var remove = (arr, el) => {
  const i = arr.indexOf(el);
  if (i > -1) {
    arr.splice(i, 1);
  }
};
var hasOwnProperty = Object.prototype.hasOwnProperty;
var hasOwn = (val, key) => hasOwnProperty.call(val, key);
var isArray = Array.isArray;
var isMap = (val) => toTypeString(val) === "[object Map]";
var isSet = (val) => toTypeString(val) === "[object Set]";
var isDate = (val) => toTypeString(val) === "[object Date]";
var isFunction = (val) => typeof val === "function";
var isString = (val) => typeof val === "string";
var isSymbol = (val) => typeof val === "symbol";
var isObject = (val) => val !== null && typeof val === "object";
var isPromise = (val) => {
  return (isObject(val) || isFunction(val)) && isFunction(val.then) && isFunction(val.catch);
};
var objectToString = Object.prototype.toString;
var toTypeString = (value) => objectToString.call(value);
var toRawType = (value) => {
  return toTypeString(value).slice(8, -1);
};
var isPlainObject = (val) => toTypeString(val) === "[object Object]";
var isIntegerKey = (key) => isString(key) && key !== "NaN" && key[0] !== "-" && "" + parseInt(key, 10) === key;
var isReservedProp = /* @__PURE__ */ makeMap(",key,ref,ref_for,ref_key,onVnodeBeforeMount,onVnodeMounted,onVnodeBeforeUpdate,onVnodeUpdated,onVnodeBeforeUnmount,onVnodeUnmounted");
var cacheStringFunction = (fn) => {
  const cache = /* @__PURE__ */ Object.create(null);
  return (str) => {
    const hit = cache[str];
    return hit || (cache[str] = fn(str));
  };
};
var camelizeRE = /-\w/g;
var camelize = cacheStringFunction((str) => {
  return str.replace(camelizeRE, (c) => c.slice(1).toUpperCase());
});
var hyphenateRE = /\B([A-Z])/g;
var hyphenate = cacheStringFunction((str) => str.replace(hyphenateRE, "-$1").toLowerCase());
var capitalize = cacheStringFunction((str) => {
  return str.charAt(0).toUpperCase() + str.slice(1);
});
var toHandlerKey = cacheStringFunction((str) => {
  const s = str ? `on${capitalize(str)}` : ``;
  return s;
});
var hasChanged = (value, oldValue) => !Object.is(value, oldValue);
var invokeArrayFns = (fns, ...arg) => {
  for (let i = 0;i < fns.length; i++) {
    fns[i](...arg);
  }
};
var def = (obj, key, value, writable = false) => {
  Object.defineProperty(obj, key, {
    configurable: true,
    enumerable: false,
    writable,
    value
  });
};
var looseToNumber = (val) => {
  const n = parseFloat(val);
  return isNaN(n) ? val : n;
};
var _globalThis;
var getGlobalThis = () => {
  return _globalThis || (_globalThis = typeof globalThis !== "undefined" ? globalThis : typeof self !== "undefined" ? self : typeof window !== "undefined" ? window : typeof global !== "undefined" ? global : {});
};
function normalizeStyle(value) {
  if (isArray(value)) {
    const res = {};
    for (let i = 0;i < value.length; i++) {
      const item = value[i];
      const normalized = isString(item) ? parseStringStyle(item) : normalizeStyle(item);
      if (normalized) {
        for (const key in normalized) {
          res[key] = normalized[key];
        }
      }
    }
    return res;
  } else if (isString(value) || isObject(value)) {
    return value;
  }
}
var listDelimiterRE = /;(?![^(]*\))/g;
var propertyDelimiterRE = /:([^]+)/;
var styleCommentRE = /\/\*[^]*?\*\//g;
function parseStringStyle(cssText) {
  const ret = {};
  cssText.replace(styleCommentRE, "").split(listDelimiterRE).forEach((item) => {
    if (item) {
      const tmp = item.split(propertyDelimiterRE);
      tmp.length > 1 && (ret[tmp[0].trim()] = tmp[1].trim());
    }
  });
  return ret;
}
function normalizeClass(value) {
  let res = "";
  if (isString(value)) {
    res = value;
  } else if (isArray(value)) {
    for (let i = 0;i < value.length; i++) {
      const normalized = normalizeClass(value[i]);
      if (normalized) {
        res += normalized + " ";
      }
    }
  } else if (isObject(value)) {
    for (const name in value) {
      if (value[name]) {
        res += name + " ";
      }
    }
  }
  return res.trim();
}
var specialBooleanAttrs = `itemscope,allowfullscreen,formnovalidate,ismap,nomodule,novalidate,readonly`;
var isSpecialBooleanAttr = /* @__PURE__ */ makeMap(specialBooleanAttrs);
var isBooleanAttr = /* @__PURE__ */ makeMap(specialBooleanAttrs + `,async,autofocus,autoplay,controls,default,defer,disabled,inert,loop,open,required,reversed,scoped,seamless,checked,muted,multiple,selected`);
function includeBooleanAttr(value) {
  return !!value || value === "";
}
function looseCompareArrays(a, b) {
  if (a.length !== b.length)
    return false;
  let equal = true;
  for (let i = 0;equal && i < a.length; i++) {
    equal = looseEqual(a[i], b[i]);
  }
  return equal;
}
function looseEqual(a, b) {
  if (a === b)
    return true;
  let aValidType = isDate(a);
  let bValidType = isDate(b);
  if (aValidType || bValidType) {
    return aValidType && bValidType ? a.getTime() === b.getTime() : false;
  }
  aValidType = isSymbol(a);
  bValidType = isSymbol(b);
  if (aValidType || bValidType) {
    return a === b;
  }
  aValidType = isArray(a);
  bValidType = isArray(b);
  if (aValidType || bValidType) {
    return aValidType && bValidType ? looseCompareArrays(a, b) : false;
  }
  aValidType = isObject(a);
  bValidType = isObject(b);
  if (aValidType || bValidType) {
    if (!aValidType || !bValidType) {
      return false;
    }
    const aKeysCount = Object.keys(a).length;
    const bKeysCount = Object.keys(b).length;
    if (aKeysCount !== bKeysCount) {
      return false;
    }
    for (const key in a) {
      const aHasKey = a.hasOwnProperty(key);
      const bHasKey = b.hasOwnProperty(key);
      if (aHasKey && !bHasKey || !aHasKey && bHasKey || !looseEqual(a[key], b[key])) {
        return false;
      }
    }
  }
  return String(a) === String(b);
}
function looseIndexOf(arr, val) {
  return arr.findIndex((item) => looseEqual(item, val));
}
var isRef = (val) => {
  return !!(val && val["__v_isRef"] === true);
};
var toDisplayString = (val) => {
  return isString(val) ? val : val == null ? "" : isArray(val) || isObject(val) && (val.toString === objectToString || !isFunction(val.toString)) ? isRef(val) ? toDisplayString(val.value) : JSON.stringify(val, replacer, 2) : String(val);
};
var replacer = (_key, val) => {
  if (isRef(val)) {
    return replacer(_key, val.value);
  } else if (isMap(val)) {
    return {
      [`Map(${val.size})`]: [...val.entries()].reduce((entries, [key, val2], i) => {
        entries[stringifySymbol(key, i) + " =>"] = val2;
        return entries;
      }, {})
    };
  } else if (isSet(val)) {
    return {
      [`Set(${val.size})`]: [...val.values()].map((v) => stringifySymbol(v))
    };
  } else if (isSymbol(val)) {
    return stringifySymbol(val);
  } else if (isObject(val) && !isArray(val) && !isPlainObject(val)) {
    return String(val);
  }
  return val;
};
var stringifySymbol = (v, i = "") => {
  var _a;
  return isSymbol(v) ? `Symbol(${(_a = v.description) != null ? _a : i})` : v;
};

// node_modules/@vue/reactivity/dist/reactivity.esm-bundler.js
function warn(msg, ...args) {
  console.warn(`[Vue warn] ${msg}`, ...args);
}
var activeEffectScope;

class EffectScope {
  constructor(detached = false) {
    this.detached = detached;
    this._active = true;
    this._on = 0;
    this.effects = [];
    this.cleanups = [];
    this._isPaused = false;
    this._warnOnRun = true;
    this.__v_skip = true;
    if (!detached && activeEffectScope) {
      if (activeEffectScope.active) {
        this.parent = activeEffectScope;
        this.index = (activeEffectScope.scopes || (activeEffectScope.scopes = [])).push(this) - 1;
      } else {
        this._active = false;
        this._warnOnRun = false;
      }
    }
  }
  get active() {
    return this._active;
  }
  pause() {
    if (this._active) {
      this._isPaused = true;
      let i, l;
      if (this.scopes) {
        const scopes = this.scopes.slice();
        for (i = 0, l = scopes.length;i < l; i++) {
          scopes[i].pause();
        }
      }
      for (i = 0, l = this.effects.length;i < l; i++) {
        this.effects[i].pause();
      }
    }
  }
  resume() {
    if (this._active) {
      if (this._isPaused) {
        this._isPaused = false;
        let i, l;
        if (this.scopes) {
          const scopes = this.scopes.slice();
          for (i = 0, l = scopes.length;i < l; i++) {
            scopes[i].resume();
          }
        }
        const effects = this.effects.slice();
        for (i = 0, l = effects.length;i < l; i++) {
          effects[i].resume();
        }
      }
    }
  }
  run(fn) {
    if (this._active) {
      const currentEffectScope = activeEffectScope;
      try {
        activeEffectScope = this;
        return fn();
      } finally {
        activeEffectScope = currentEffectScope;
      }
    } else if (false) {}
  }
  on() {
    if (++this._on === 1) {
      this.prevScope = activeEffectScope;
      activeEffectScope = this;
    }
  }
  off() {
    if (this._on > 0 && --this._on === 0) {
      if (activeEffectScope === this) {
        activeEffectScope = this.prevScope;
      } else {
        let current = activeEffectScope;
        while (current) {
          if (current.prevScope === this) {
            current.prevScope = this.prevScope;
            break;
          }
          current = current.prevScope;
        }
      }
      this.prevScope = undefined;
    }
  }
  stop(fromParent) {
    if (this._active) {
      this._active = false;
      let i, l;
      for (i = 0, l = this.effects.length;i < l; i++) {
        this.effects[i].stop();
      }
      this.effects.length = 0;
      for (i = 0, l = this.cleanups.length;i < l; i++) {
        this.cleanups[i]();
      }
      this.cleanups.length = 0;
      if (this.scopes) {
        const scopes = this.scopes.slice();
        for (i = 0, l = scopes.length;i < l; i++) {
          scopes[i].stop(true);
        }
        this.scopes.length = 0;
      }
      if (!this.detached && this.parent && !fromParent) {
        const last = this.parent.scopes.pop();
        if (last && last !== this) {
          this.parent.scopes[this.index] = last;
          last.index = this.index;
        }
      }
      this.parent = undefined;
    }
  }
}
function getCurrentScope() {
  return activeEffectScope;
}
var activeSub;
var pausedQueueEffects = /* @__PURE__ */ new WeakSet;

class ReactiveEffect {
  constructor(fn) {
    this.fn = fn;
    this.deps = undefined;
    this.depsTail = undefined;
    this.flags = 1 | 4;
    this.next = undefined;
    this.cleanup = undefined;
    this.scheduler = undefined;
    if (activeEffectScope) {
      if (activeEffectScope.active) {
        activeEffectScope.effects.push(this);
      } else {
        this.flags &= -2;
      }
    }
  }
  pause() {
    this.flags |= 64;
  }
  resume() {
    if (this.flags & 64) {
      this.flags &= -65;
      if (pausedQueueEffects.has(this)) {
        pausedQueueEffects.delete(this);
        this.trigger();
      }
    }
  }
  notify() {
    if (this.flags & 2 && !(this.flags & 32)) {
      return;
    }
    if (!(this.flags & 8)) {
      batch(this);
    }
  }
  run() {
    if (!(this.flags & 1)) {
      return this.fn();
    }
    this.flags |= 2;
    cleanupEffect(this);
    prepareDeps(this);
    const prevEffect = activeSub;
    const prevShouldTrack = shouldTrack;
    activeSub = this;
    shouldTrack = true;
    try {
      return this.fn();
    } finally {
      if (false) {}
      cleanupDeps(this);
      activeSub = prevEffect;
      shouldTrack = prevShouldTrack;
      this.flags &= -3;
    }
  }
  stop() {
    if (this.flags & 1) {
      for (let link = this.deps;link; link = link.nextDep) {
        removeSub(link);
      }
      this.deps = this.depsTail = undefined;
      cleanupEffect(this);
      this.onStop && this.onStop();
      this.flags &= -2;
    }
  }
  trigger() {
    if (this.flags & 64) {
      pausedQueueEffects.add(this);
    } else if (this.scheduler) {
      this.scheduler();
    } else {
      this.runIfDirty();
    }
  }
  runIfDirty() {
    if (isDirty(this)) {
      this.run();
    }
  }
  get dirty() {
    return isDirty(this);
  }
}
var batchDepth = 0;
var batchedSub;
var batchedComputed;
function batch(sub, isComputed = false) {
  sub.flags |= 8;
  if (isComputed) {
    sub.next = batchedComputed;
    batchedComputed = sub;
    return;
  }
  sub.next = batchedSub;
  batchedSub = sub;
}
function startBatch() {
  batchDepth++;
}
function endBatch() {
  if (--batchDepth > 0) {
    return;
  }
  if (batchedComputed) {
    let e = batchedComputed;
    batchedComputed = undefined;
    while (e) {
      const next = e.next;
      e.next = undefined;
      e.flags &= -9;
      e = next;
    }
  }
  let error;
  while (batchedSub) {
    let e = batchedSub;
    batchedSub = undefined;
    while (e) {
      const next = e.next;
      e.next = undefined;
      e.flags &= -9;
      if (e.flags & 1) {
        try {
          e.trigger();
        } catch (err) {
          if (!error)
            error = err;
        }
      }
      e = next;
    }
  }
  if (error)
    throw error;
}
function prepareDeps(sub) {
  for (let link = sub.deps;link; link = link.nextDep) {
    link.version = -1;
    link.prevActiveLink = link.dep.activeLink;
    link.dep.activeLink = link;
  }
}
function cleanupDeps(sub) {
  let head;
  let tail = sub.depsTail;
  let link = tail;
  while (link) {
    const prev = link.prevDep;
    if (link.version === -1) {
      if (link === tail)
        tail = prev;
      removeSub(link);
      removeDep(link);
    } else {
      head = link;
    }
    link.dep.activeLink = link.prevActiveLink;
    link.prevActiveLink = undefined;
    link = prev;
  }
  sub.deps = head;
  sub.depsTail = tail;
}
function isDirty(sub) {
  for (let link = sub.deps;link; link = link.nextDep) {
    if (link.dep.version !== link.version || link.dep.computed && (refreshComputed(link.dep.computed) || link.dep.version !== link.version)) {
      return true;
    }
  }
  if (sub._dirty) {
    return true;
  }
  return false;
}
function refreshComputed(computed) {
  if (computed.flags & 4 && !(computed.flags & 16)) {
    return;
  }
  computed.flags &= -17;
  if (computed.globalVersion === globalVersion) {
    return;
  }
  computed.globalVersion = globalVersion;
  if (!computed.isSSR && computed.flags & 128 && (!computed.deps && !computed._dirty || !isDirty(computed))) {
    return;
  }
  computed.flags |= 2;
  const dep = computed.dep;
  const prevSub = activeSub;
  const prevShouldTrack = shouldTrack;
  activeSub = computed;
  shouldTrack = true;
  try {
    prepareDeps(computed);
    const value = computed.fn(computed._value);
    if (dep.version === 0 || hasChanged(value, computed._value)) {
      computed.flags |= 128;
      computed._value = value;
      dep.version++;
    }
  } catch (err) {
    dep.version++;
    throw err;
  } finally {
    activeSub = prevSub;
    shouldTrack = prevShouldTrack;
    cleanupDeps(computed);
    computed.flags &= -3;
  }
}
function removeSub(link, soft = false) {
  const { dep, prevSub, nextSub } = link;
  if (prevSub) {
    prevSub.nextSub = nextSub;
    link.prevSub = undefined;
  }
  if (nextSub) {
    nextSub.prevSub = prevSub;
    link.nextSub = undefined;
  }
  if (false) {}
  if (dep.subs === link) {
    dep.subs = prevSub;
    if (!prevSub && dep.computed) {
      dep.computed.flags &= -5;
      for (let l = dep.computed.deps;l; l = l.nextDep) {
        removeSub(l, true);
      }
    }
  }
  if (!soft && !--dep.sc && dep.map) {
    dep.map.delete(dep.key);
  }
}
function removeDep(link) {
  const { prevDep, nextDep } = link;
  if (prevDep) {
    prevDep.nextDep = nextDep;
    link.prevDep = undefined;
  }
  if (nextDep) {
    nextDep.prevDep = prevDep;
    link.nextDep = undefined;
  }
}
var shouldTrack = true;
var trackStack = [];
function pauseTracking() {
  trackStack.push(shouldTrack);
  shouldTrack = false;
}
function resetTracking() {
  const last = trackStack.pop();
  shouldTrack = last === undefined ? true : last;
}
function cleanupEffect(e) {
  const { cleanup } = e;
  e.cleanup = undefined;
  if (cleanup) {
    const prevSub = activeSub;
    activeSub = undefined;
    try {
      cleanup();
    } finally {
      activeSub = prevSub;
    }
  }
}
var globalVersion = 0;

class Link {
  constructor(sub, dep) {
    this.sub = sub;
    this.dep = dep;
    this.version = dep.version;
    this.nextDep = this.prevDep = this.nextSub = this.prevSub = this.prevActiveLink = undefined;
  }
}

class Dep {
  constructor(computed) {
    this.computed = computed;
    this.version = 0;
    this.activeLink = undefined;
    this.subs = undefined;
    this.map = undefined;
    this.key = undefined;
    this.sc = 0;
    this.__v_skip = true;
    if (false) {}
  }
  track(debugInfo) {
    if (!activeSub || !shouldTrack || activeSub === this.computed) {
      return;
    }
    let link = this.activeLink;
    if (link === undefined || link.sub !== activeSub) {
      link = this.activeLink = new Link(activeSub, this);
      if (!activeSub.deps) {
        activeSub.deps = activeSub.depsTail = link;
      } else {
        link.prevDep = activeSub.depsTail;
        activeSub.depsTail.nextDep = link;
        activeSub.depsTail = link;
      }
      addSub(link);
    } else if (link.version === -1) {
      link.version = this.version;
      if (link.nextDep) {
        const next = link.nextDep;
        next.prevDep = link.prevDep;
        if (link.prevDep) {
          link.prevDep.nextDep = next;
        }
        link.prevDep = activeSub.depsTail;
        link.nextDep = undefined;
        activeSub.depsTail.nextDep = link;
        activeSub.depsTail = link;
        if (activeSub.deps === link) {
          activeSub.deps = next;
        }
      }
    }
    if (false) {}
    return link;
  }
  trigger(debugInfo) {
    this.version++;
    globalVersion++;
    this.notify(debugInfo);
  }
  notify(debugInfo) {
    startBatch();
    try {
      if (false) {}
      for (let link = this.subs;link; link = link.prevSub) {
        if (link.sub.notify()) {
          link.sub.dep.notify();
        }
      }
    } finally {
      endBatch();
    }
  }
}
function addSub(link) {
  link.dep.sc++;
  if (link.sub.flags & 4) {
    const computed = link.dep.computed;
    if (computed && !link.dep.subs) {
      computed.flags |= 4 | 16;
      for (let l = computed.deps;l; l = l.nextDep) {
        addSub(l);
      }
    }
    const currentTail = link.dep.subs;
    if (currentTail !== link) {
      link.prevSub = currentTail;
      if (currentTail)
        currentTail.nextSub = link;
    }
    if (false) {}
    link.dep.subs = link;
  }
}
var targetMap = /* @__PURE__ */ new WeakMap;
var ITERATE_KEY = /* @__PURE__ */ Symbol("");
var MAP_KEY_ITERATE_KEY = /* @__PURE__ */ Symbol("");
var ARRAY_ITERATE_KEY = /* @__PURE__ */ Symbol("");
function track(target, type, key) {
  if (shouldTrack && activeSub) {
    let depsMap = targetMap.get(target);
    if (!depsMap) {
      targetMap.set(target, depsMap = /* @__PURE__ */ new Map);
    }
    let dep = depsMap.get(key);
    if (!dep) {
      depsMap.set(key, dep = new Dep);
      dep.map = depsMap;
      dep.key = key;
    }
    if (false) {} else {
      dep.track();
    }
  }
}
function trigger(target, type, key, newValue, oldValue, oldTarget) {
  const depsMap = targetMap.get(target);
  if (!depsMap) {
    globalVersion++;
    return;
  }
  const run = (dep) => {
    if (dep) {
      if (false) {} else {
        dep.trigger();
      }
    }
  };
  startBatch();
  if (type === "clear") {
    depsMap.forEach(run);
  } else {
    const targetIsArray = isArray(target);
    const isArrayIndex = targetIsArray && isIntegerKey(key);
    if (targetIsArray && key === "length") {
      const newLength = Number(newValue);
      depsMap.forEach((dep, key2) => {
        if (key2 === "length" || key2 === ARRAY_ITERATE_KEY || !isSymbol(key2) && key2 >= newLength) {
          run(dep);
        }
      });
    } else {
      if (key !== undefined || depsMap.has(undefined)) {
        run(depsMap.get(key));
      }
      if (isArrayIndex) {
        run(depsMap.get(ARRAY_ITERATE_KEY));
      }
      switch (type) {
        case "add":
          if (!targetIsArray) {
            run(depsMap.get(ITERATE_KEY));
            if (isMap(target)) {
              run(depsMap.get(MAP_KEY_ITERATE_KEY));
            }
          } else if (isArrayIndex) {
            run(depsMap.get("length"));
          }
          break;
        case "delete":
          if (!targetIsArray) {
            run(depsMap.get(ITERATE_KEY));
            if (isMap(target)) {
              run(depsMap.get(MAP_KEY_ITERATE_KEY));
            }
          }
          break;
        case "set":
          if (isMap(target)) {
            run(depsMap.get(ITERATE_KEY));
          }
          break;
      }
    }
  }
  endBatch();
}
function reactiveReadArray(array) {
  const raw = toRaw(array);
  if (raw === array)
    return raw;
  track(raw, "iterate", ARRAY_ITERATE_KEY);
  return isShallow(array) ? raw : raw.map(toReactive);
}
function shallowReadArray(arr) {
  track(arr = toRaw(arr), "iterate", ARRAY_ITERATE_KEY);
  return arr;
}
function toWrapped(target, item) {
  if (isReadonly(target)) {
    return isReactive(target) ? toReadonly(toReactive(item)) : toReadonly(item);
  }
  return toReactive(item);
}
var arrayInstrumentations = {
  __proto__: null,
  [Symbol.iterator]() {
    return iterator(this, Symbol.iterator, (item) => toWrapped(this, item));
  },
  concat(...args) {
    return reactiveReadArray(this).concat(...args.map((x) => isArray(x) ? reactiveReadArray(x) : x));
  },
  entries() {
    return iterator(this, "entries", (value) => {
      value[1] = toWrapped(this, value[1]);
      return value;
    });
  },
  every(fn, thisArg) {
    return apply(this, "every", fn, thisArg, undefined, arguments);
  },
  filter(fn, thisArg) {
    return apply(this, "filter", fn, thisArg, (v) => v.map((item) => toWrapped(this, item)), arguments);
  },
  find(fn, thisArg) {
    return apply(this, "find", fn, thisArg, (item) => toWrapped(this, item), arguments);
  },
  findIndex(fn, thisArg) {
    return apply(this, "findIndex", fn, thisArg, undefined, arguments);
  },
  findLast(fn, thisArg) {
    return apply(this, "findLast", fn, thisArg, (item) => toWrapped(this, item), arguments);
  },
  findLastIndex(fn, thisArg) {
    return apply(this, "findLastIndex", fn, thisArg, undefined, arguments);
  },
  forEach(fn, thisArg) {
    return apply(this, "forEach", fn, thisArg, undefined, arguments);
  },
  includes(...args) {
    return searchProxy(this, "includes", args);
  },
  indexOf(...args) {
    return searchProxy(this, "indexOf", args);
  },
  join(separator) {
    return reactiveReadArray(this).join(separator);
  },
  lastIndexOf(...args) {
    return searchProxy(this, "lastIndexOf", args);
  },
  map(fn, thisArg) {
    return apply(this, "map", fn, thisArg, undefined, arguments);
  },
  pop() {
    return noTracking(this, "pop");
  },
  push(...args) {
    return noTracking(this, "push", args);
  },
  reduce(fn, ...args) {
    return reduce(this, "reduce", fn, args);
  },
  reduceRight(fn, ...args) {
    return reduce(this, "reduceRight", fn, args);
  },
  shift() {
    return noTracking(this, "shift");
  },
  some(fn, thisArg) {
    return apply(this, "some", fn, thisArg, undefined, arguments);
  },
  splice(...args) {
    return noTracking(this, "splice", args);
  },
  toReversed() {
    return reactiveReadArray(this).toReversed();
  },
  toSorted(comparer) {
    return reactiveReadArray(this).toSorted(comparer);
  },
  toSpliced(...args) {
    return reactiveReadArray(this).toSpliced(...args);
  },
  unshift(...args) {
    return noTracking(this, "unshift", args);
  },
  values() {
    return iterator(this, "values", (item) => toWrapped(this, item));
  }
};
function iterator(self2, method, wrapValue) {
  const arr = shallowReadArray(self2);
  const iter = arr[method]();
  if (arr !== self2 && !isShallow(self2)) {
    iter._next = iter.next;
    iter.next = () => {
      const result = iter._next();
      if (!result.done) {
        result.value = wrapValue(result.value);
      }
      return result;
    };
  }
  return iter;
}
var arrayProto = Array.prototype;
function apply(self2, method, fn, thisArg, wrappedRetFn, args) {
  const arr = shallowReadArray(self2);
  const needsWrap = arr !== self2 && !isShallow(self2);
  const methodFn = arr[method];
  if (methodFn !== arrayProto[method]) {
    const result2 = methodFn.apply(self2, args);
    return needsWrap ? toReactive(result2) : result2;
  }
  let wrappedFn = fn;
  if (arr !== self2) {
    if (needsWrap) {
      wrappedFn = function(item, index) {
        return fn.call(this, toWrapped(self2, item), index, self2);
      };
    } else if (fn.length > 2) {
      wrappedFn = function(item, index) {
        return fn.call(this, item, index, self2);
      };
    }
  }
  const result = methodFn.call(arr, wrappedFn, thisArg);
  return needsWrap && wrappedRetFn ? wrappedRetFn(result) : result;
}
function reduce(self2, method, fn, args) {
  const arr = shallowReadArray(self2);
  const needsWrap = arr !== self2 && !isShallow(self2);
  let wrappedFn = fn;
  let wrapInitialAccumulator = false;
  if (arr !== self2) {
    if (needsWrap) {
      wrapInitialAccumulator = args.length === 0;
      wrappedFn = function(acc, item, index) {
        if (wrapInitialAccumulator) {
          wrapInitialAccumulator = false;
          acc = toWrapped(self2, acc);
        }
        return fn.call(this, acc, toWrapped(self2, item), index, self2);
      };
    } else if (fn.length > 3) {
      wrappedFn = function(acc, item, index) {
        return fn.call(this, acc, item, index, self2);
      };
    }
  }
  const result = arr[method](wrappedFn, ...args);
  return wrapInitialAccumulator ? toWrapped(self2, result) : result;
}
function searchProxy(self2, method, args) {
  const arr = toRaw(self2);
  track(arr, "iterate", ARRAY_ITERATE_KEY);
  const res = arr[method](...args);
  if ((res === -1 || res === false) && isProxy(args[0])) {
    args[0] = toRaw(args[0]);
    return arr[method](...args);
  }
  return res;
}
function noTracking(self2, method, args = []) {
  pauseTracking();
  startBatch();
  const res = toRaw(self2)[method].apply(self2, args);
  endBatch();
  resetTracking();
  return res;
}
var isNonTrackableKeys = /* @__PURE__ */ makeMap(`__proto__,__v_isRef,__isVue`);
var builtInSymbols = new Set(/* @__PURE__ */ Object.getOwnPropertyNames(Symbol).filter((key) => key !== "arguments" && key !== "caller").map((key) => Symbol[key]).filter(isSymbol));
function hasOwnProperty2(key) {
  if (!isSymbol(key))
    key = String(key);
  const obj = toRaw(this);
  track(obj, "has", key);
  return obj.hasOwnProperty(key);
}

class BaseReactiveHandler {
  constructor(_isReadonly = false, _isShallow = false) {
    this._isReadonly = _isReadonly;
    this._isShallow = _isShallow;
  }
  get(target, key, receiver) {
    if (key === "__v_skip")
      return target["__v_skip"];
    const isReadonly2 = this._isReadonly, isShallow2 = this._isShallow;
    if (key === "__v_isReactive") {
      return !isReadonly2;
    } else if (key === "__v_isReadonly") {
      return isReadonly2;
    } else if (key === "__v_isShallow") {
      return isShallow2;
    } else if (key === "__v_raw") {
      if (receiver === (isReadonly2 ? isShallow2 ? shallowReadonlyMap : readonlyMap : isShallow2 ? shallowReactiveMap : reactiveMap).get(target) || Object.getPrototypeOf(target) === Object.getPrototypeOf(receiver)) {
        return target;
      }
      return;
    }
    const targetIsArray = isArray(target);
    if (!isReadonly2) {
      let fn;
      if (targetIsArray && (fn = arrayInstrumentations[key])) {
        return fn;
      }
      if (key === "hasOwnProperty") {
        return hasOwnProperty2;
      }
    }
    const res = Reflect.get(target, key, isRef2(target) ? target : receiver);
    if (isSymbol(key) ? builtInSymbols.has(key) : isNonTrackableKeys(key)) {
      return res;
    }
    if (!isReadonly2) {
      track(target, "get", key);
    }
    if (isShallow2) {
      return res;
    }
    if (isRef2(res)) {
      const value = targetIsArray && isIntegerKey(key) ? res : res.value;
      return isReadonly2 && isObject(value) ? readonly(value) : value;
    }
    if (isObject(res)) {
      return isReadonly2 ? readonly(res) : reactive(res);
    }
    return res;
  }
}

class MutableReactiveHandler extends BaseReactiveHandler {
  constructor(isShallow2 = false) {
    super(false, isShallow2);
  }
  set(target, key, value, receiver) {
    let oldValue = target[key];
    const isArrayWithIntegerKey = isArray(target) && isIntegerKey(key);
    if (!this._isShallow) {
      const isOldValueReadonly = isReadonly(oldValue);
      if (!isShallow(value) && !isReadonly(value)) {
        oldValue = toRaw(oldValue);
        value = toRaw(value);
      }
      if (!isArrayWithIntegerKey && isRef2(oldValue) && !isRef2(value)) {
        if (isOldValueReadonly) {
          if (false) {}
          return true;
        } else {
          oldValue.value = value;
          return true;
        }
      }
    }
    const hadKey = isArrayWithIntegerKey ? Number(key) < target.length : hasOwn(target, key);
    const result = Reflect.set(target, key, value, isRef2(target) ? target : receiver);
    if (target === toRaw(receiver) && result) {
      if (!hadKey) {
        trigger(target, "add", key, value);
      } else if (hasChanged(value, oldValue)) {
        trigger(target, "set", key, value, oldValue);
      }
    }
    return result;
  }
  deleteProperty(target, key) {
    const hadKey = hasOwn(target, key);
    const oldValue = target[key];
    const result = Reflect.deleteProperty(target, key);
    if (result && hadKey) {
      trigger(target, "delete", key, undefined, oldValue);
    }
    return result;
  }
  has(target, key) {
    const result = Reflect.has(target, key);
    if (!isSymbol(key) || !builtInSymbols.has(key)) {
      track(target, "has", key);
    }
    return result;
  }
  ownKeys(target) {
    track(target, "iterate", isArray(target) ? "length" : ITERATE_KEY);
    return Reflect.ownKeys(target);
  }
}

class ReadonlyReactiveHandler extends BaseReactiveHandler {
  constructor(isShallow2 = false) {
    super(true, isShallow2);
  }
  set(target, key) {
    if (false) {}
    return true;
  }
  deleteProperty(target, key) {
    if (false) {}
    return true;
  }
}
var mutableHandlers = /* @__PURE__ */ new MutableReactiveHandler;
var readonlyHandlers = /* @__PURE__ */ new ReadonlyReactiveHandler;
var shallowReactiveHandlers = /* @__PURE__ */ new MutableReactiveHandler(true);
var toShallow = (value) => value;
var getProto = (v) => Reflect.getPrototypeOf(v);
function createIterableMethod(method, isReadonly2, isShallow2) {
  return function(...args) {
    const target = this["__v_raw"];
    const rawTarget = toRaw(target);
    const targetIsMap = isMap(rawTarget);
    const isPair = method === "entries" || method === Symbol.iterator && targetIsMap;
    const isKeyOnly = method === "keys" && targetIsMap;
    const innerIterator = target[method](...args);
    const wrap = isShallow2 ? toShallow : isReadonly2 ? toReadonly : toReactive;
    !isReadonly2 && track(rawTarget, "iterate", isKeyOnly ? MAP_KEY_ITERATE_KEY : ITERATE_KEY);
    return extend(Object.create(innerIterator), {
      next() {
        const { value, done } = innerIterator.next();
        return done ? { value, done } : {
          value: isPair ? [wrap(value[0]), wrap(value[1])] : wrap(value),
          done
        };
      }
    });
  };
}
function createReadonlyMethod(type) {
  return function(...args) {
    if (false) {}
    return type === "delete" ? false : type === "clear" ? undefined : this;
  };
}
function createInstrumentations(readonly, shallow) {
  const instrumentations = {
    get(key) {
      const target = this["__v_raw"];
      const rawTarget = toRaw(target);
      const rawKey = toRaw(key);
      if (!readonly) {
        if (hasChanged(key, rawKey)) {
          track(rawTarget, "get", key);
        }
        track(rawTarget, "get", rawKey);
      }
      const { has } = getProto(rawTarget);
      const wrap = shallow ? toShallow : readonly ? toReadonly : toReactive;
      if (has.call(rawTarget, key)) {
        return wrap(target.get(key));
      } else if (has.call(rawTarget, rawKey)) {
        return wrap(target.get(rawKey));
      } else if (target !== rawTarget) {
        target.get(key);
      }
    },
    get size() {
      const target = this["__v_raw"];
      !readonly && track(toRaw(target), "iterate", ITERATE_KEY);
      return target.size;
    },
    has(key) {
      const target = this["__v_raw"];
      const rawTarget = toRaw(target);
      const rawKey = toRaw(key);
      if (!readonly) {
        if (hasChanged(key, rawKey)) {
          track(rawTarget, "has", key);
        }
        track(rawTarget, "has", rawKey);
      }
      return key === rawKey ? target.has(key) : target.has(key) || target.has(rawKey);
    },
    forEach(callback, thisArg) {
      const observed = this;
      const target = observed["__v_raw"];
      const rawTarget = toRaw(target);
      const wrap = shallow ? toShallow : readonly ? toReadonly : toReactive;
      !readonly && track(rawTarget, "iterate", ITERATE_KEY);
      return target.forEach((value, key) => {
        return callback.call(thisArg, wrap(value), wrap(key), observed);
      });
    }
  };
  extend(instrumentations, readonly ? {
    add: createReadonlyMethod("add"),
    set: createReadonlyMethod("set"),
    delete: createReadonlyMethod("delete"),
    clear: createReadonlyMethod("clear")
  } : {
    add(value) {
      const target = toRaw(this);
      const proto = getProto(target);
      const rawValue = toRaw(value);
      const valueToAdd = !shallow && !isShallow(value) && !isReadonly(value) ? rawValue : value;
      const hadKey = proto.has.call(target, valueToAdd) || hasChanged(value, valueToAdd) && proto.has.call(target, value) || hasChanged(rawValue, valueToAdd) && proto.has.call(target, rawValue);
      if (!hadKey) {
        target.add(valueToAdd);
        trigger(target, "add", valueToAdd, valueToAdd);
      }
      return this;
    },
    set(key, value) {
      if (!shallow && !isShallow(value) && !isReadonly(value)) {
        value = toRaw(value);
      }
      const target = toRaw(this);
      const { has, get } = getProto(target);
      let hadKey = has.call(target, key);
      if (!hadKey) {
        key = toRaw(key);
        hadKey = has.call(target, key);
      } else if (false) {}
      const oldValue = get.call(target, key);
      target.set(key, value);
      if (!hadKey) {
        trigger(target, "add", key, value);
      } else if (hasChanged(value, oldValue)) {
        trigger(target, "set", key, value, oldValue);
      }
      return this;
    },
    delete(key) {
      const target = toRaw(this);
      const { has, get } = getProto(target);
      let hadKey = has.call(target, key);
      if (!hadKey) {
        key = toRaw(key);
        hadKey = has.call(target, key);
      } else if (false) {}
      const oldValue = get ? get.call(target, key) : undefined;
      const result = target.delete(key);
      if (hadKey) {
        trigger(target, "delete", key, undefined, oldValue);
      }
      return result;
    },
    clear() {
      const target = toRaw(this);
      const hadItems = target.size !== 0;
      const oldTarget = undefined;
      const result = target.clear();
      if (hadItems) {
        trigger(target, "clear", undefined, undefined, oldTarget);
      }
      return result;
    }
  });
  const iteratorMethods = [
    "keys",
    "values",
    "entries",
    Symbol.iterator
  ];
  iteratorMethods.forEach((method) => {
    instrumentations[method] = createIterableMethod(method, readonly, shallow);
  });
  return instrumentations;
}
function createInstrumentationGetter(isReadonly2, shallow) {
  const instrumentations = createInstrumentations(isReadonly2, shallow);
  return (target, key, receiver) => {
    if (key === "__v_isReactive") {
      return !isReadonly2;
    } else if (key === "__v_isReadonly") {
      return isReadonly2;
    } else if (key === "__v_raw") {
      return target;
    }
    return Reflect.get(hasOwn(instrumentations, key) && key in target ? instrumentations : target, key, receiver);
  };
}
var mutableCollectionHandlers = {
  get: /* @__PURE__ */ createInstrumentationGetter(false, false)
};
var shallowCollectionHandlers = {
  get: /* @__PURE__ */ createInstrumentationGetter(false, true)
};
var readonlyCollectionHandlers = {
  get: /* @__PURE__ */ createInstrumentationGetter(true, false)
};
var reactiveMap = /* @__PURE__ */ new WeakMap;
var shallowReactiveMap = /* @__PURE__ */ new WeakMap;
var readonlyMap = /* @__PURE__ */ new WeakMap;
var shallowReadonlyMap = /* @__PURE__ */ new WeakMap;
function targetTypeMap(rawType) {
  switch (rawType) {
    case "Object":
    case "Array":
      return 1;
    case "Map":
    case "Set":
    case "WeakMap":
    case "WeakSet":
      return 2;
    default:
      return 0;
  }
}
function reactive(target) {
  if (/* @__PURE__ */ isReadonly(target)) {
    return target;
  }
  return createReactiveObject(target, false, mutableHandlers, mutableCollectionHandlers, reactiveMap);
}
function shallowReactive(target) {
  return createReactiveObject(target, false, shallowReactiveHandlers, shallowCollectionHandlers, shallowReactiveMap);
}
function readonly(target) {
  return createReactiveObject(target, true, readonlyHandlers, readonlyCollectionHandlers, readonlyMap);
}
function createReactiveObject(target, isReadonly2, baseHandlers, collectionHandlers, proxyMap) {
  if (!isObject(target)) {
    if (false) {}
    return target;
  }
  if (target["__v_raw"] && !(isReadonly2 && target["__v_isReactive"])) {
    return target;
  }
  if (target["__v_skip"] || !Object.isExtensible(target)) {
    return target;
  }
  const existingProxy = proxyMap.get(target);
  if (existingProxy) {
    return existingProxy;
  }
  const targetType = targetTypeMap(toRawType(target));
  if (targetType === 0) {
    return target;
  }
  const proxy = new Proxy(target, targetType === 2 ? collectionHandlers : baseHandlers);
  proxyMap.set(target, proxy);
  return proxy;
}
function isReactive(value) {
  if (/* @__PURE__ */ isReadonly(value)) {
    return /* @__PURE__ */ isReactive(value["__v_raw"]);
  }
  return !!(value && value["__v_isReactive"]);
}
function isReadonly(value) {
  return !!(value && value["__v_isReadonly"]);
}
function isShallow(value) {
  return !!(value && value["__v_isShallow"]);
}
function isProxy(value) {
  return value ? !!value["__v_raw"] : false;
}
function toRaw(observed) {
  const raw = observed && observed["__v_raw"];
  return raw ? /* @__PURE__ */ toRaw(raw) : observed;
}
function markRaw(value) {
  if (!hasOwn(value, "__v_skip") && Object.isExtensible(value)) {
    def(value, "__v_skip", true);
  }
  return value;
}
var toReactive = (value) => isObject(value) ? /* @__PURE__ */ reactive(value) : value;
var toReadonly = (value) => isObject(value) ? /* @__PURE__ */ readonly(value) : value;
function isRef2(r) {
  return r ? r["__v_isRef"] === true : false;
}
function ref(value) {
  return createRef(value, false);
}
function shallowRef(value) {
  return createRef(value, true);
}
function createRef(rawValue, shallow) {
  if (/* @__PURE__ */ isRef2(rawValue)) {
    return rawValue;
  }
  return new RefImpl(rawValue, shallow);
}

class RefImpl {
  constructor(value, isShallow2) {
    this.dep = new Dep;
    this["__v_isRef"] = true;
    this["__v_isShallow"] = false;
    this._rawValue = isShallow2 ? value : toRaw(value);
    this._value = isShallow2 ? value : toReactive(value);
    this["__v_isShallow"] = isShallow2;
  }
  get value() {
    if (false) {} else {
      this.dep.track();
    }
    return this._value;
  }
  set value(newValue) {
    const oldValue = this._rawValue;
    const useDirectValue = this["__v_isShallow"] || isShallow(newValue) || isReadonly(newValue);
    newValue = useDirectValue ? newValue : toRaw(newValue);
    if (hasChanged(newValue, oldValue)) {
      this._rawValue = newValue;
      this._value = useDirectValue ? newValue : toReactive(newValue);
      if (false) {} else {
        this.dep.trigger();
      }
    }
  }
}
function unref(ref2) {
  return /* @__PURE__ */ isRef2(ref2) ? ref2.value : ref2;
}
var shallowUnwrapHandlers = {
  get: (target, key, receiver) => key === "__v_raw" ? target : unref(Reflect.get(target, key, receiver)),
  set: (target, key, value, receiver) => {
    const oldValue = target[key];
    if (/* @__PURE__ */ isRef2(oldValue) && !/* @__PURE__ */ isRef2(value)) {
      oldValue.value = value;
      return true;
    } else {
      return Reflect.set(target, key, value, receiver);
    }
  }
};
function proxyRefs(objectWithRefs) {
  return isReactive(objectWithRefs) ? objectWithRefs : new Proxy(objectWithRefs, shallowUnwrapHandlers);
}
class ComputedRefImpl {
  constructor(fn, setter, isSSR) {
    this.fn = fn;
    this.setter = setter;
    this._value = undefined;
    this.dep = new Dep(this);
    this.__v_isRef = true;
    this.deps = undefined;
    this.depsTail = undefined;
    this.flags = 16;
    this.globalVersion = globalVersion - 1;
    this.next = undefined;
    this.effect = this;
    this["__v_isReadonly"] = !setter;
    this.isSSR = isSSR;
  }
  notify() {
    this.flags |= 16;
    if (!(this.flags & 8) && activeSub !== this) {
      batch(this, true);
      return true;
    } else if (false)
      ;
  }
  get value() {
    const link = this.dep.track();
    refreshComputed(this);
    if (link) {
      link.version = this.dep.version;
    }
    return this._value;
  }
  set value(newValue) {
    if (this.setter) {
      this.setter(newValue);
    } else if (false) {}
  }
}
function computed(getterOrOptions, debugOptions, isSSR = false) {
  let getter;
  let setter;
  if (isFunction(getterOrOptions)) {
    getter = getterOrOptions;
  } else {
    getter = getterOrOptions.get;
    setter = getterOrOptions.set;
  }
  const cRef = new ComputedRefImpl(getter, setter, isSSR);
  if (false) {}
  return cRef;
}
var INITIAL_WATCHER_VALUE = {};
var cleanupMap = /* @__PURE__ */ new WeakMap;
var activeWatcher = undefined;
function onWatcherCleanup(cleanupFn, failSilently = false, owner = activeWatcher) {
  if (owner) {
    let cleanups = cleanupMap.get(owner);
    if (!cleanups)
      cleanupMap.set(owner, cleanups = []);
    cleanups.push(cleanupFn);
  } else if (false) {}
}
function watch(source, cb, options = EMPTY_OBJ) {
  const { immediate, deep, once, scheduler, augmentJob, call } = options;
  const warnInvalidSource = (s) => {
    (options.onWarn || warn)(`Invalid watch source: `, s, `A watch source can only be a getter/effect function, a ref, a reactive object, or an array of these types.`);
  };
  const reactiveGetter = (source2) => {
    if (deep)
      return source2;
    if (isShallow(source2) || deep === false || deep === 0)
      return traverse(source2, 1);
    return traverse(source2);
  };
  let effect;
  let getter;
  let cleanup;
  let boundCleanup;
  let forceTrigger = false;
  let isMultiSource = false;
  if (isRef2(source)) {
    getter = () => source.value;
    forceTrigger = isShallow(source);
  } else if (isReactive(source)) {
    getter = () => reactiveGetter(source);
    forceTrigger = true;
  } else if (isArray(source)) {
    isMultiSource = true;
    forceTrigger = source.some((s) => isReactive(s) || isShallow(s));
    getter = () => source.map((s) => {
      if (isRef2(s)) {
        return s.value;
      } else if (isReactive(s)) {
        return reactiveGetter(s);
      } else if (isFunction(s)) {
        return call ? call(s, 2) : s();
      }
    });
  } else if (isFunction(source)) {
    if (cb) {
      getter = call ? () => call(source, 2) : source;
    } else {
      getter = () => {
        if (cleanup) {
          pauseTracking();
          try {
            cleanup();
          } finally {
            resetTracking();
          }
        }
        const currentEffect = activeWatcher;
        activeWatcher = effect;
        try {
          return call ? call(source, 3, [boundCleanup]) : source(boundCleanup);
        } finally {
          activeWatcher = currentEffect;
        }
      };
    }
  } else {
    getter = NOOP;
  }
  if (cb && deep) {
    const baseGetter = getter;
    const depth = deep === true ? Infinity : deep;
    getter = () => traverse(baseGetter(), depth);
  }
  const scope = getCurrentScope();
  const watchHandle = () => {
    effect.stop();
    if (scope && scope.active) {
      remove(scope.effects, effect);
    }
  };
  if (once && cb) {
    const _cb = cb;
    cb = (...args) => {
      const res = _cb(...args);
      watchHandle();
      return res;
    };
  }
  let oldValue = isMultiSource ? new Array(source.length).fill(INITIAL_WATCHER_VALUE) : INITIAL_WATCHER_VALUE;
  const job = (immediateFirstRun) => {
    if (!(effect.flags & 1) || !effect.dirty && !immediateFirstRun) {
      return;
    }
    if (cb) {
      const newValue = effect.run();
      if (immediateFirstRun || deep || forceTrigger || (isMultiSource ? newValue.some((v, i) => hasChanged(v, oldValue[i])) : hasChanged(newValue, oldValue))) {
        if (cleanup) {
          cleanup();
        }
        const currentWatcher = activeWatcher;
        activeWatcher = effect;
        try {
          const args = [
            newValue,
            oldValue === INITIAL_WATCHER_VALUE ? undefined : isMultiSource && oldValue[0] === INITIAL_WATCHER_VALUE ? [] : oldValue,
            boundCleanup
          ];
          oldValue = newValue;
          call ? call(cb, 3, args) : cb(...args);
        } finally {
          activeWatcher = currentWatcher;
        }
      }
    } else {
      effect.run();
    }
  };
  if (augmentJob) {
    augmentJob(job);
  }
  effect = new ReactiveEffect(getter);
  effect.scheduler = scheduler ? () => scheduler(job, false) : job;
  boundCleanup = (fn) => onWatcherCleanup(fn, false, effect);
  cleanup = effect.onStop = () => {
    const cleanups = cleanupMap.get(effect);
    if (cleanups) {
      if (call) {
        call(cleanups, 4);
      } else {
        for (const cleanup2 of cleanups)
          cleanup2();
      }
      cleanupMap.delete(effect);
    }
  };
  if (false) {}
  if (cb) {
    if (immediate) {
      job(true);
    } else {
      oldValue = effect.run();
    }
  } else if (scheduler) {
    scheduler(job.bind(null, true), true);
  } else {
    effect.run();
  }
  watchHandle.pause = effect.pause.bind(effect);
  watchHandle.resume = effect.resume.bind(effect);
  watchHandle.stop = watchHandle;
  return watchHandle;
}
function traverse(value, depth = Infinity, seen) {
  if (depth <= 0 || !isObject(value) || value["__v_skip"]) {
    return value;
  }
  seen = seen || /* @__PURE__ */ new Map;
  if ((seen.get(value) || 0) >= depth) {
    return value;
  }
  seen.set(value, depth);
  depth--;
  if (isRef2(value)) {
    traverse(value.value, depth, seen);
  } else if (isArray(value)) {
    for (let i = 0;i < value.length; i++) {
      traverse(value[i], depth, seen);
    }
  } else if (isSet(value) || isMap(value)) {
    value.forEach((v) => {
      traverse(v, depth, seen);
    });
  } else if (isPlainObject(value)) {
    for (const key in value) {
      traverse(value[key], depth, seen);
    }
    for (const key of Object.getOwnPropertySymbols(value)) {
      if (Object.prototype.propertyIsEnumerable.call(value, key)) {
        traverse(value[key], depth, seen);
      }
    }
  }
  return value;
}
// node_modules/@vue/runtime-core/dist/runtime-core.esm-bundler.js
function callWithErrorHandling(fn, instance, type, args) {
  try {
    return args ? fn(...args) : fn();
  } catch (err) {
    handleError(err, instance, type);
  }
}
function callWithAsyncErrorHandling(fn, instance, type, args) {
  if (isFunction(fn)) {
    const res = callWithErrorHandling(fn, instance, type, args);
    if (res && isPromise(res)) {
      res.catch((err) => {
        handleError(err, instance, type);
      });
    }
    return res;
  }
  if (isArray(fn)) {
    const values = [];
    for (let i = 0;i < fn.length; i++) {
      values.push(callWithAsyncErrorHandling(fn[i], instance, type, args));
    }
    return values;
  } else if (false) {}
}
function handleError(err, instance, type, throwInDev = true) {
  const contextVNode = instance ? instance.vnode : null;
  const { errorHandler, throwUnhandledErrorInProduction } = instance && instance.appContext.config || EMPTY_OBJ;
  if (instance) {
    let cur = instance.parent;
    const exposedInstance = instance.proxy;
    const errorInfo = `https://vuejs.org/error-reference/#runtime-${type}`;
    while (cur) {
      const errorCapturedHooks = cur.ec;
      if (errorCapturedHooks) {
        for (let i = 0;i < errorCapturedHooks.length; i++) {
          if (errorCapturedHooks[i](err, exposedInstance, errorInfo) === false) {
            return;
          }
        }
      }
      cur = cur.parent;
    }
    if (errorHandler) {
      pauseTracking();
      callWithErrorHandling(errorHandler, null, 10, [
        err,
        exposedInstance,
        errorInfo
      ]);
      resetTracking();
      return;
    }
  }
  logError(err, type, contextVNode, throwInDev, throwUnhandledErrorInProduction);
}
function logError(err, type, contextVNode, throwInDev = true, throwInProd = false) {
  if (false) {} else if (throwInProd) {
    throw err;
  } else {
    console.error(err);
  }
}
var queue = [];
var flushIndex = -1;
var pendingPostFlushCbs = [];
var activePostFlushCbs = null;
var postFlushIndex = 0;
var resolvedPromise = /* @__PURE__ */ Promise.resolve();
var currentFlushPromise = null;
function nextTick(fn) {
  const p = currentFlushPromise || resolvedPromise;
  return fn ? p.then(this ? fn.bind(this) : fn) : p;
}
function findInsertionIndex(id) {
  let start = flushIndex + 1;
  let end = queue.length;
  while (start < end) {
    const middle = start + end >>> 1;
    const middleJob = queue[middle];
    const middleJobId = getId(middleJob);
    if (middleJobId < id || middleJobId === id && middleJob.flags & 2) {
      start = middle + 1;
    } else {
      end = middle;
    }
  }
  return start;
}
function queueJob(job) {
  if (!(job.flags & 1)) {
    const jobId = getId(job);
    const lastJob = queue[queue.length - 1];
    if (!lastJob || !(job.flags & 2) && jobId >= getId(lastJob)) {
      queue.push(job);
    } else {
      queue.splice(findInsertionIndex(jobId), 0, job);
    }
    job.flags |= 1;
    queueFlush();
  }
}
function queueFlush() {
  if (!currentFlushPromise) {
    currentFlushPromise = resolvedPromise.then(flushJobs);
  }
}
function queuePostFlushCb(cb) {
  if (!isArray(cb)) {
    if (activePostFlushCbs && cb.id === -1) {
      activePostFlushCbs.splice(postFlushIndex + 1, 0, cb);
    } else if (!(cb.flags & 1)) {
      pendingPostFlushCbs.push(cb);
      cb.flags |= 1;
    }
  } else {
    for (let i = 0;i < cb.length; i++) {
      pendingPostFlushCbs.push(cb[i]);
    }
  }
  queueFlush();
}
function flushPreFlushCbs(instance, seen, i = flushIndex + 1) {
  if (false) {}
  for (;i < queue.length; i++) {
    const cb = queue[i];
    if (cb && cb.flags & 2) {
      if (instance && cb.id !== instance.uid) {
        continue;
      }
      if (false) {}
      queue.splice(i, 1);
      i--;
      if (cb.flags & 4) {
        cb.flags &= -2;
      }
      cb();
      if (!(cb.flags & 4)) {
        cb.flags &= -2;
      }
    }
  }
}
function flushPostFlushCbs(seen) {
  if (pendingPostFlushCbs.length) {
    const deduped = [...new Set(pendingPostFlushCbs)].sort((a, b) => getId(a) - getId(b));
    pendingPostFlushCbs.length = 0;
    if (activePostFlushCbs) {
      for (let i = 0;i < deduped.length; i++) {
        activePostFlushCbs.push(deduped[i]);
      }
      return;
    }
    activePostFlushCbs = deduped;
    if (false) {}
    for (postFlushIndex = 0;postFlushIndex < activePostFlushCbs.length; postFlushIndex++) {
      const cb = activePostFlushCbs[postFlushIndex];
      if (false) {}
      if (cb.flags & 4) {
        cb.flags &= -2;
      }
      if (!(cb.flags & 8))
        cb();
      cb.flags &= -2;
    }
    activePostFlushCbs = null;
    postFlushIndex = 0;
  }
}
var getId = (job) => job.id == null ? job.flags & 2 ? -1 : Infinity : job.id;
function flushJobs(seen) {
  if (false) {}
  const check = NOOP;
  try {
    for (flushIndex = 0;flushIndex < queue.length; flushIndex++) {
      const job = queue[flushIndex];
      if (job && !(job.flags & 8)) {
        if (false) {}
        if (job.flags & 4) {
          job.flags &= ~1;
        }
        callWithErrorHandling(job, job.i, job.i ? 15 : 14);
        if (!(job.flags & 4)) {
          job.flags &= ~1;
        }
      }
    }
  } finally {
    for (;flushIndex < queue.length; flushIndex++) {
      const job = queue[flushIndex];
      if (job) {
        job.flags &= -2;
      }
    }
    flushIndex = -1;
    queue.length = 0;
    flushPostFlushCbs(seen);
    currentFlushPromise = null;
    if (queue.length || pendingPostFlushCbs.length) {
      flushJobs(seen);
    }
  }
}
if (false) {}
var currentRenderingInstance = null;
var currentScopeId = null;
function setCurrentRenderingInstance(instance) {
  const prev = currentRenderingInstance;
  currentRenderingInstance = instance;
  currentScopeId = instance && instance.type.__scopeId || null;
  return prev;
}
function withCtx(fn, ctx = currentRenderingInstance, isNonScopedSlot) {
  if (!ctx)
    return fn;
  if (fn._n) {
    return fn;
  }
  const renderFnWithContext = (...args) => {
    if (renderFnWithContext._d) {
      setBlockTracking(-1);
    }
    const prevInstance = setCurrentRenderingInstance(ctx);
    const prevStackSize = blockStack.length;
    let res;
    try {
      res = fn(...args);
    } finally {
      for (let i = blockStack.length;i > prevStackSize; i--)
        closeBlock();
      setCurrentRenderingInstance(prevInstance);
      if (renderFnWithContext._d) {
        setBlockTracking(1);
      }
    }
    if (false) {}
    return res;
  };
  renderFnWithContext._n = true;
  renderFnWithContext._c = true;
  renderFnWithContext._d = true;
  return renderFnWithContext;
}
function withDirectives(vnode, directives) {
  if (currentRenderingInstance === null) {
    return vnode;
  }
  const instance = getComponentPublicInstance(currentRenderingInstance);
  const bindings = vnode.dirs || (vnode.dirs = []);
  for (let i = 0;i < directives.length; i++) {
    let [dir, value, arg, modifiers = EMPTY_OBJ] = directives[i];
    if (dir) {
      if (isFunction(dir)) {
        dir = {
          mounted: dir,
          updated: dir
        };
      }
      if (dir.deep) {
        traverse(value);
      }
      bindings.push({
        dir,
        instance,
        value,
        oldValue: undefined,
        arg,
        modifiers
      });
    }
  }
  return vnode;
}
function invokeDirectiveHook(vnode, prevVNode, instance, name) {
  const bindings = vnode.dirs;
  const oldBindings = prevVNode && prevVNode.dirs;
  for (let i = 0;i < bindings.length; i++) {
    const binding = bindings[i];
    if (oldBindings) {
      binding.oldValue = oldBindings[i].value;
    }
    let hook = binding.dir[name];
    if (hook) {
      pauseTracking();
      callWithAsyncErrorHandling(hook, instance, 8, [
        vnode.el,
        binding,
        vnode,
        prevVNode
      ]);
      resetTracking();
    }
  }
}
function inject(key, defaultValue, treatDefaultAsFactory = false) {
  const instance = getCurrentInstance();
  if (instance || currentApp) {
    let provides = currentApp ? currentApp._context.provides : instance ? instance.parent == null || instance.ce ? instance.vnode.appContext && instance.vnode.appContext.provides : instance.parent.provides : undefined;
    if (provides && key in provides) {
      return provides[key];
    } else if (arguments.length > 1) {
      return treatDefaultAsFactory && isFunction(defaultValue) ? defaultValue.call(instance && instance.proxy) : defaultValue;
    } else if (false) {}
  } else if (false) {}
}
var ssrContextKey = /* @__PURE__ */ Symbol.for("v-scx");
var useSSRContext = () => {
  {
    const ctx = inject(ssrContextKey);
    if (!ctx) {}
    return ctx;
  }
};
function watch2(source, cb, options) {
  if (false) {}
  return doWatch(source, cb, options);
}
function doWatch(source, cb, options = EMPTY_OBJ) {
  const { immediate, deep, flush, once } = options;
  if (false) {}
  const baseWatchOptions = extend({}, options);
  if (false)
    ;
  const runsImmediately = cb && immediate || !cb && flush !== "post";
  let ssrCleanup;
  if (isInSSRComponentSetup) {
    if (flush === "sync") {
      const ctx = useSSRContext();
      ssrCleanup = ctx.__watcherHandles || (ctx.__watcherHandles = []);
    } else if (!runsImmediately) {
      const watchStopHandle = () => {};
      watchStopHandle.stop = NOOP;
      watchStopHandle.resume = NOOP;
      watchStopHandle.pause = NOOP;
      return watchStopHandle;
    }
  }
  const instance = currentInstance;
  baseWatchOptions.call = (fn, type, args) => callWithAsyncErrorHandling(fn, instance, type, args);
  let isPre = false;
  if (flush === "post") {
    baseWatchOptions.scheduler = (job) => {
      queuePostRenderEffect(job, instance && instance.suspense);
    };
  } else if (flush !== "sync") {
    isPre = true;
    baseWatchOptions.scheduler = (job, isFirstRun) => {
      if (isFirstRun) {
        job();
      } else {
        queueJob(job);
      }
    };
  }
  baseWatchOptions.augmentJob = (job) => {
    if (cb) {
      job.flags |= 4;
    }
    if (isPre) {
      job.flags |= 2;
      if (instance) {
        job.id = instance.uid;
        job.i = instance;
      }
    }
  };
  const watchHandle = watch(source, cb, baseWatchOptions);
  if (isInSSRComponentSetup) {
    if (ssrCleanup) {
      ssrCleanup.push(watchHandle);
    } else if (runsImmediately) {
      watchHandle();
    }
  }
  return watchHandle;
}
var TeleportEndKey = /* @__PURE__ */ Symbol("_vte");
var isTeleport = (type) => type.__isTeleport;
var leaveCbKey = /* @__PURE__ */ Symbol("_leaveCb");
function findNonCommentChild(children) {
  let child = children[0];
  if (children.length > 1) {
    let hasFound = false;
    for (const c of children) {
      if (c.type !== Comment) {
        if (false) {}
        child = c;
        hasFound = true;
        if (true)
          break;
      }
    }
  }
  return child;
}
function getInnerChild$1(vnode) {
  if (!isKeepAlive(vnode)) {
    if (isTeleport(vnode.type) && vnode.children) {
      return findNonCommentChild(vnode.children);
    }
    return vnode;
  }
  if (vnode.component) {
    return vnode.component.subTree;
  }
  const { shapeFlag, children } = vnode;
  if (children) {
    if (shapeFlag & 16) {
      return children[0];
    }
    if (shapeFlag & 32 && isFunction(children.default)) {
      return children.default();
    }
  }
}
function setTransitionHooks(vnode, hooks) {
  if (vnode.shapeFlag & 6 && vnode.component) {
    vnode.transition = hooks;
    const subTree = vnode.component.subTree;
    setTransitionHooks(isTeleport(subTree.type) ? getInnerChild$1(subTree) || subTree : subTree, hooks);
  } else if (vnode.shapeFlag & 128) {
    vnode.ssContent.transition = hooks.clone(vnode.ssContent);
    vnode.ssFallback.transition = hooks.clone(vnode.ssFallback);
  } else {
    vnode.transition = hooks;
  }
}
function defineComponent(options, extraOptions) {
  return isFunction(options) ? /* @__PURE__ */ (() => extend({ name: options.name }, extraOptions, { setup: options }))() : options;
}
function markAsyncBoundary(instance) {
  instance.ids = [instance.ids[0] + instance.ids[2]++ + "-", 0, 0];
}
function useTemplateRef(key) {
  const i = getCurrentInstance();
  const r = shallowRef(null);
  if (i) {
    const refs = i.refs === EMPTY_OBJ ? i.refs = {} : i.refs;
    if (false) {} else {
      Object.defineProperty(refs, key, {
        enumerable: true,
        get: () => r.value,
        set: (val) => r.value = val
      });
    }
  } else if (false) {}
  const ret = r;
  if (false) {}
  return ret;
}
function isTemplateRefKey(refs, key) {
  let desc;
  return !!((desc = Object.getOwnPropertyDescriptor(refs, key)) && !desc.configurable);
}
var pendingSetRefMap = /* @__PURE__ */ new WeakMap;
function setRef(rawRef, oldRawRef, parentSuspense, vnode, isUnmount = false) {
  if (isArray(rawRef)) {
    rawRef.forEach((r, i) => setRef(r, oldRawRef && (isArray(oldRawRef) ? oldRawRef[i] : oldRawRef), parentSuspense, vnode, isUnmount));
    return;
  }
  if (isAsyncWrapper(vnode) && !isUnmount) {
    if (vnode.shapeFlag & 512 && vnode.type.__asyncResolved && vnode.component.subTree.component) {
      setRef(rawRef, oldRawRef, parentSuspense, vnode.component.subTree);
    }
    return;
  }
  const refValue = vnode.shapeFlag & 4 ? getComponentPublicInstance(vnode.component) : vnode.el;
  const value = isUnmount ? null : refValue;
  const { i: owner, r: ref2 } = rawRef;
  if (false) {}
  const oldRef = oldRawRef && oldRawRef.r;
  const refs = owner.refs === EMPTY_OBJ ? owner.refs = {} : owner.refs;
  const setupState = owner.setupState;
  const rawSetupState = toRaw(setupState);
  const canSetSetupRef = setupState === EMPTY_OBJ ? NO : (key) => {
    if (false) {}
    if (isTemplateRefKey(refs, key)) {
      return false;
    }
    return hasOwn(rawSetupState, key);
  };
  const canSetRef = (ref22, key) => {
    if (false) {}
    if (key && isTemplateRefKey(refs, key)) {
      return false;
    }
    return true;
  };
  if (oldRef != null && oldRef !== ref2) {
    invalidatePendingSetRef(oldRawRef);
    if (isString(oldRef)) {
      refs[oldRef] = null;
      if (canSetSetupRef(oldRef)) {
        setupState[oldRef] = null;
      }
    } else if (isRef2(oldRef)) {
      const oldRawRefAtom = oldRawRef;
      if (canSetRef(oldRef, oldRawRefAtom.k)) {
        oldRef.value = null;
      }
      if (oldRawRefAtom.k)
        refs[oldRawRefAtom.k] = null;
    }
  }
  if (isFunction(ref2)) {
    callWithErrorHandling(ref2, owner, 12, [value, refs]);
  } else {
    const _isString = isString(ref2);
    const _isRef = isRef2(ref2);
    if (_isString || _isRef) {
      const doSet = () => {
        if (rawRef.f) {
          const existing = _isString ? canSetSetupRef(ref2) ? setupState[ref2] : refs[ref2] : canSetRef(ref2) || !rawRef.k ? ref2.value : refs[rawRef.k];
          if (isUnmount) {
            isArray(existing) && remove(existing, refValue);
          } else {
            if (!isArray(existing)) {
              if (_isString) {
                refs[ref2] = [refValue];
                if (canSetSetupRef(ref2)) {
                  setupState[ref2] = refs[ref2];
                }
              } else {
                const newVal = [refValue];
                if (canSetRef(ref2, rawRef.k)) {
                  ref2.value = newVal;
                }
                if (rawRef.k)
                  refs[rawRef.k] = newVal;
              }
            } else if (!existing.includes(refValue)) {
              existing.push(refValue);
            }
          }
        } else if (_isString) {
          refs[ref2] = value;
          if (canSetSetupRef(ref2)) {
            setupState[ref2] = value;
          }
        } else if (_isRef) {
          if (canSetRef(ref2, rawRef.k)) {
            ref2.value = value;
          }
          if (rawRef.k)
            refs[rawRef.k] = value;
        } else if (false) {}
      };
      if (value) {
        const job = () => {
          doSet();
          pendingSetRefMap.delete(rawRef);
        };
        job.id = -1;
        pendingSetRefMap.set(rawRef, job);
        queuePostRenderEffect(job, parentSuspense);
      } else {
        invalidatePendingSetRef(rawRef);
        doSet();
      }
    } else if (false) {}
  }
}
function invalidatePendingSetRef(rawRef) {
  const pendingSetRef = pendingSetRefMap.get(rawRef);
  if (pendingSetRef) {
    pendingSetRef.flags |= 8;
    pendingSetRefMap.delete(rawRef);
  }
}
var requestIdleCallback = getGlobalThis().requestIdleCallback || ((cb) => setTimeout(cb, 1));
var cancelIdleCallback = getGlobalThis().cancelIdleCallback || ((id) => clearTimeout(id));
var isAsyncWrapper = (i) => !!i.type.__asyncLoader;
var isKeepAlive = (vnode) => vnode.type.__isKeepAlive;
function injectHook(type, hook, target = currentInstance, prepend = false) {
  if (target) {
    const hooks = target[type] || (target[type] = []);
    const wrappedHook = hook.__weh || (hook.__weh = (...args) => {
      pauseTracking();
      const reset = setCurrentInstance(target);
      const res = callWithAsyncErrorHandling(hook, target, type, args);
      reset();
      resetTracking();
      return res;
    });
    if (prepend) {
      hooks.unshift(wrappedHook);
    } else {
      hooks.push(wrappedHook);
    }
    return wrappedHook;
  } else if (false) {}
}
var createHook = (lifecycle) => (hook, target = currentInstance) => {
  if (!isInSSRComponentSetup || lifecycle === "sp") {
    injectHook(lifecycle, (...args) => hook(...args), target);
  }
};
var onBeforeMount = createHook("bm");
var onMounted = createHook("m");
var onBeforeUpdate = createHook("bu");
var onUpdated = createHook("u");
var onBeforeUnmount = createHook("bum");
var onUnmounted = createHook("um");
var onServerPrefetch = createHook("sp");
var onRenderTriggered = createHook("rtg");
var onRenderTracked = createHook("rtc");
var COMPONENTS = "components";
var NULL_DYNAMIC_COMPONENT = /* @__PURE__ */ Symbol.for("v-ndc");
function resolveDynamicComponent(component) {
  if (isString(component)) {
    return resolveAsset(COMPONENTS, component, false) || component;
  } else {
    return component || NULL_DYNAMIC_COMPONENT;
  }
}
function resolveAsset(type, name, warnMissing = true, maybeSelfReference = false) {
  const instance = currentRenderingInstance || currentInstance;
  if (instance) {
    const Component = instance.type;
    if (type === COMPONENTS) {
      const selfName = getComponentName(Component, false);
      if (selfName && (selfName === name || selfName === camelize(name) || selfName === capitalize(camelize(name)))) {
        return Component;
      }
    }
    const res = resolve(instance[type] || Component[type], name) || resolve(instance.appContext[type], name);
    if (!res && maybeSelfReference) {
      return Component;
    }
    if (false) {}
    return res;
  } else if (false) {}
}
function resolve(registry, name) {
  return registry && (registry[name] || registry[camelize(name)] || registry[capitalize(camelize(name))]);
}
function renderList(source, renderItem, cache, index) {
  let ret;
  const cached = cache && cache[index];
  const sourceIsArray = isArray(source);
  if (sourceIsArray || isString(source)) {
    const sourceIsReactiveArray = sourceIsArray && isReactive(source);
    let needsWrap = false;
    let isReadonlySource = false;
    if (sourceIsReactiveArray) {
      needsWrap = !isShallow(source);
      isReadonlySource = isReadonly(source);
      source = shallowReadArray(source);
    }
    ret = new Array(source.length);
    for (let i = 0, l = source.length;i < l; i++) {
      ret[i] = renderItem(needsWrap ? isReadonlySource ? toReadonly(toReactive(source[i])) : toReactive(source[i]) : source[i], i, undefined, cached && cached[i]);
    }
  } else if (typeof source === "number") {
    if (false) {} else {
      ret = new Array(source);
      for (let i = 0;i < source; i++) {
        ret[i] = renderItem(i + 1, i, undefined, cached && cached[i]);
      }
    }
  } else if (isObject(source)) {
    if (source[Symbol.iterator]) {
      ret = Array.from(source, (item, i) => renderItem(item, i, undefined, cached && cached[i]));
    } else {
      const keys = Object.keys(source);
      ret = new Array(keys.length);
      for (let i = 0, l = keys.length;i < l; i++) {
        const key = keys[i];
        ret[i] = renderItem(source[key], key, i, cached && cached[i]);
      }
    }
  } else {
    ret = [];
  }
  if (cache) {
    cache[index] = ret;
  }
  return ret;
}
function renderSlot(slots, name, props, fallback, noSlotted, branchKey) {
  if (props == null)
    props = {};
  if (currentRenderingInstance.ce || currentRenderingInstance.parent && isAsyncWrapper(currentRenderingInstance.parent) && currentRenderingInstance.parent.ce) {
    const slotProps = branchKey != null && props.key == null ? extend({}, props, { key: branchKey }) : props;
    const hasProps = Object.keys(slotProps).length > 0;
    if (name !== "default")
      slotProps.name = name;
    return openBlock(), createBlock(Fragment, null, [createVNode("slot", slotProps, fallback && fallback())], hasProps ? -2 : 64);
  }
  let slot = slots[name];
  if (false) {}
  if (slot && slot._c) {
    slot._d = false;
  }
  const prevStackSize = blockStack.length;
  openBlock();
  let rendered;
  try {
    const validSlotContent = slot && ensureValidVNode(slot(props));
    const slotKey = props.key || branchKey || validSlotContent && validSlotContent.key;
    rendered = createBlock(Fragment, {
      key: (slotKey && !isSymbol(slotKey) ? slotKey : `_${name}`) + (!validSlotContent && fallback ? "_fb" : "")
    }, validSlotContent || (fallback ? fallback() : []), validSlotContent && slots._ === 1 ? 64 : -2);
  } catch (err) {
    for (let i = blockStack.length;i > prevStackSize; i--)
      closeBlock();
    throw err;
  } finally {
    if (slot && slot._c) {
      slot._d = true;
    }
  }
  if (!noSlotted && rendered.scopeId) {
    rendered.slotScopeIds = [rendered.scopeId + "-s"];
  }
  return rendered;
}
function ensureValidVNode(vnodes) {
  return vnodes.some((child) => {
    if (!isVNode(child))
      return true;
    if (child.type === Comment)
      return false;
    if (child.type === Fragment && !ensureValidVNode(child.children))
      return false;
    return true;
  }) ? vnodes : null;
}
var getPublicInstance = (i) => {
  if (!i)
    return null;
  if (isStatefulComponent(i))
    return getComponentPublicInstance(i);
  return getPublicInstance(i.parent);
};
var publicPropertiesMap = /* @__PURE__ */ extend(/* @__PURE__ */ Object.create(null), {
  $: (i) => i,
  $el: (i) => i.vnode.el,
  $data: (i) => i.data,
  $props: (i) => i.props,
  $attrs: (i) => i.attrs,
  $slots: (i) => i.slots,
  $refs: (i) => i.refs,
  $parent: (i) => getPublicInstance(i.parent),
  $root: (i) => getPublicInstance(i.root),
  $host: (i) => i.ce,
  $emit: (i) => i.emit,
  $options: (i) => i.type,
  $forceUpdate: (i) => i.f || (i.f = () => {
    queueJob(i.update);
  }),
  $nextTick: (i) => i.n || (i.n = nextTick.bind(i.proxy)),
  $watch: (i) => NOOP
});
var hasSetupBinding = (state, key) => state !== EMPTY_OBJ && !state.__isScriptSetup && hasOwn(state, key);
var PublicInstanceProxyHandlers = {
  get({ _: instance }, key) {
    if (key === "__v_skip") {
      return true;
    }
    const { ctx, setupState, data, props, accessCache, type, appContext } = instance;
    if (false) {}
    if (key[0] !== "$") {
      const n = accessCache[key];
      if (n !== undefined) {
        switch (n) {
          case 1:
            return setupState[key];
          case 2:
            return data[key];
          case 4:
            return ctx[key];
          case 3:
            return props[key];
        }
      } else if (hasSetupBinding(setupState, key)) {
        accessCache[key] = 1;
        return setupState[key];
      } else if (false) {} else if (hasOwn(props, key)) {
        accessCache[key] = 3;
        return props[key];
      } else if (ctx !== EMPTY_OBJ && hasOwn(ctx, key)) {
        accessCache[key] = 4;
        return ctx[key];
      } else if (true) {
        accessCache[key] = 0;
      }
    }
    const publicGetter = publicPropertiesMap[key];
    let cssModule, globalProperties;
    if (publicGetter) {
      if (key === "$attrs") {
        track(instance.attrs, "get", "");
      } else if (false) {}
      return publicGetter(instance);
    } else if ((cssModule = type.__cssModules) && (cssModule = cssModule[key])) {
      return cssModule;
    } else if (ctx !== EMPTY_OBJ && hasOwn(ctx, key)) {
      accessCache[key] = 4;
      return ctx[key];
    } else if (globalProperties = appContext.config.globalProperties, hasOwn(globalProperties, key)) {
      {
        return globalProperties[key];
      }
    } else if (false) {}
  },
  set({ _: instance }, key, value) {
    const { data, setupState, ctx } = instance;
    if (hasSetupBinding(setupState, key)) {
      setupState[key] = value;
      return true;
    } else if (false) {} else if (false) {} else if (hasOwn(instance.props, key)) {
      return false;
    }
    if (key[0] === "$" && key.slice(1) in instance) {
      return false;
    } else {
      if (false) {} else {
        ctx[key] = value;
      }
    }
    return true;
  },
  has({
    _: { data, setupState, accessCache, ctx, appContext, props, type }
  }, key) {
    let cssModules;
    return !!(accessCache[key] || false || hasSetupBinding(setupState, key) || hasOwn(props, key) || hasOwn(ctx, key) || hasOwn(publicPropertiesMap, key) || hasOwn(appContext.config.globalProperties, key) || (cssModules = type.__cssModules) && cssModules[key]);
  },
  defineProperty(target, key, descriptor) {
    if (descriptor.get != null) {
      target._.accessCache[key] = 0;
    } else if (hasOwn(descriptor, "value")) {
      this.set(target, key, descriptor.value, null);
    }
    return Reflect.defineProperty(target, key, descriptor);
  }
};
if (false) {}
function createAppContext() {
  return {
    app: null,
    config: {
      isNativeTag: NO,
      performance: false,
      globalProperties: {},
      optionMergeStrategies: {},
      errorHandler: undefined,
      warnHandler: undefined,
      compilerOptions: {}
    },
    mixins: [],
    components: {},
    directives: {},
    provides: /* @__PURE__ */ Object.create(null),
    optionsCache: /* @__PURE__ */ new WeakMap,
    propsCache: /* @__PURE__ */ new WeakMap,
    emitsCache: /* @__PURE__ */ new WeakMap
  };
}
var uid$1 = 0;
function createAppAPI(render, hydrate) {
  return function createApp(rootComponent, rootProps = null) {
    if (!isFunction(rootComponent)) {
      rootComponent = extend({}, rootComponent);
    }
    if (rootProps != null && !isObject(rootProps)) {
      rootProps = null;
    }
    const context = createAppContext();
    const installedPlugins = /* @__PURE__ */ new WeakSet;
    const pluginCleanupFns = [];
    let isMounted = false;
    const app = context.app = {
      _uid: uid$1++,
      _component: rootComponent,
      _props: rootProps,
      _container: null,
      _context: context,
      _instance: null,
      version,
      get config() {
        return context.config;
      },
      set config(v) {
        if (false) {}
      },
      use(plugin, ...options) {
        if (installedPlugins.has(plugin)) {} else if (plugin && isFunction(plugin.install)) {
          installedPlugins.add(plugin);
          plugin.install(app, ...options);
        } else if (isFunction(plugin)) {
          installedPlugins.add(plugin);
          plugin(app, ...options);
        } else if (false) {}
        return app;
      },
      mixin(mixin) {
        if (false) {} else if (false) {}
        return app;
      },
      component(name, component) {
        if (false) {}
        if (!component) {
          return context.components[name];
        }
        if (false) {}
        context.components[name] = component;
        return app;
      },
      directive(name, directive) {
        if (false) {}
        if (!directive) {
          return context.directives[name];
        }
        if (false) {}
        context.directives[name] = directive;
        return app;
      },
      mount(rootContainer, isHydrate, namespace) {
        if (!isMounted) {
          if (false) {}
          const vnode = app._ceVNode || createVNode(rootComponent, rootProps);
          vnode.appContext = context;
          if (namespace === true) {
            namespace = "svg";
          } else if (namespace === false) {
            namespace = undefined;
          }
          if (false) {}
          if (isHydrate && hydrate) {
            hydrate(vnode, rootContainer);
          } else {
            render(vnode, rootContainer, namespace);
          }
          isMounted = true;
          app._container = rootContainer;
          rootContainer.__vue_app__ = app;
          if (false) {}
          return getComponentPublicInstance(vnode.component);
        } else if (false) {}
      },
      onUnmount(cleanupFn) {
        if (false) {}
        pluginCleanupFns.push(cleanupFn);
      },
      unmount() {
        if (isMounted) {
          callWithAsyncErrorHandling(pluginCleanupFns, app._instance, 16);
          render(null, app._container);
          if (false) {}
          delete app._container.__vue_app__;
        } else if (false) {}
      },
      provide(key, value) {
        if (false) {}
        context.provides[key] = value;
        return app;
      },
      runWithContext(fn) {
        const lastApp = currentApp;
        currentApp = app;
        try {
          return fn();
        } finally {
          currentApp = lastApp;
        }
      }
    };
    return app;
  };
}
var currentApp = null;
var getModelModifiers = (props, modelName) => {
  return modelName === "modelValue" || modelName === "model-value" ? props.modelModifiers : props[`${modelName}Modifiers`] || props[`${camelize(modelName)}Modifiers`] || props[`${hyphenate(modelName)}Modifiers`];
};
function emit(instance, event, ...rawArgs) {
  if (instance.isUnmounted)
    return;
  const props = instance.vnode.props || EMPTY_OBJ;
  if (false) {}
  let args = rawArgs;
  const isModelListener2 = event.startsWith("update:");
  const modifiers = isModelListener2 && getModelModifiers(props, event.slice(7));
  if (modifiers) {
    if (modifiers.trim) {
      args = rawArgs.map((a) => isString(a) ? a.trim() : a);
    }
    if (modifiers.number) {
      args = rawArgs.map(looseToNumber);
    }
  }
  if (false) {}
  if (false) {}
  let handlerName;
  let handler = props[handlerName = toHandlerKey(event)] || props[handlerName = toHandlerKey(camelize(event))];
  if (!handler && isModelListener2) {
    handler = props[handlerName = toHandlerKey(hyphenate(event))];
  }
  if (handler) {
    callWithAsyncErrorHandling(handler, instance, 6, args);
  }
  const onceHandler = props[handlerName + `Once`];
  if (onceHandler) {
    if (!instance.emitted) {
      instance.emitted = {};
    } else if (instance.emitted[handlerName]) {
      return;
    }
    instance.emitted[handlerName] = true;
    callWithAsyncErrorHandling(onceHandler, instance, 6, args);
  }
}
function normalizeEmitsOptions(comp, appContext, asMixin = false) {
  const cache = appContext.emitsCache;
  const cached = cache.get(comp);
  if (cached !== undefined) {
    return cached;
  }
  const raw = comp.emits;
  let normalized = {};
  let hasExtends = false;
  if (false) {}
  if (!raw && !hasExtends) {
    if (isObject(comp)) {
      cache.set(comp, null);
    }
    return null;
  }
  if (isArray(raw)) {
    raw.forEach((key) => normalized[key] = null);
  } else {
    extend(normalized, raw);
  }
  if (isObject(comp)) {
    cache.set(comp, normalized);
  }
  return normalized;
}
function isEmitListener(options, key) {
  if (!options || !isOn(key)) {
    return false;
  }
  key = key.slice(2);
  key = key === "Once" ? key : key.replace(/Once$/, "");
  return hasOwn(options, key[0].toLowerCase() + key.slice(1)) || hasOwn(options, hyphenate(key)) || hasOwn(options, key);
}
function renderComponentRoot(instance) {
  const {
    type: Component,
    vnode,
    proxy,
    withProxy,
    propsOptions: [propsOptions],
    slots,
    attrs,
    emit: emit2,
    render,
    renderCache,
    props,
    data,
    setupState,
    ctx,
    inheritAttrs
  } = instance;
  const prev = setCurrentRenderingInstance(instance);
  let result;
  let fallthroughAttrs;
  if (false) {}
  try {
    if (vnode.shapeFlag & 4) {
      const proxyToUse = withProxy || proxy;
      const thisProxy = proxyToUse;
      result = normalizeVNode(render.call(thisProxy, proxyToUse, renderCache, props, setupState, data, ctx));
      fallthroughAttrs = attrs;
    } else {
      const render2 = Component;
      if (false) {}
      result = normalizeVNode(render2.length > 1 ? render2(props, { attrs, slots, emit: emit2 }) : render2(props, null));
      fallthroughAttrs = Component.props ? attrs : getFunctionalFallthrough(attrs);
    }
  } catch (err) {
    blockStack.length = 0;
    handleError(err, instance, 1);
    result = createVNode(Comment);
  }
  let root = result;
  let setRoot = undefined;
  if (false) {}
  if (fallthroughAttrs && inheritAttrs !== false) {
    const keys = Object.keys(fallthroughAttrs);
    const { shapeFlag } = root;
    if (keys.length) {
      if (shapeFlag & (1 | 6)) {
        if (propsOptions && keys.some(isModelListener)) {
          fallthroughAttrs = filterModelListeners(fallthroughAttrs, propsOptions);
        }
        root = cloneVNode(root, fallthroughAttrs, false, true);
      } else if (false) {}
    }
  }
  if (vnode.dirs) {
    if (false) {}
    root = cloneVNode(root, null, false, true);
    root.dirs = root.dirs ? root.dirs.concat(vnode.dirs) : vnode.dirs;
  }
  if (vnode.transition) {
    const child = isTeleport(root.type) ? getInnerChild$1(root) || root : root;
    if (false) {}
    setTransitionHooks(child, vnode.transition);
  }
  if (false) {} else {
    result = root;
  }
  setCurrentRenderingInstance(prev);
  return result;
}
var getFunctionalFallthrough = (attrs) => {
  let res;
  for (const key in attrs) {
    if (key === "class" || key === "style" || isOn(key)) {
      (res || (res = {}))[key] = attrs[key];
    }
  }
  return res;
};
var filterModelListeners = (attrs, props) => {
  const res = {};
  for (const key in attrs) {
    if (!isModelListener(key) || !(key.slice(9) in props)) {
      res[key] = attrs[key];
    }
  }
  return res;
};
function shouldUpdateComponent(prevVNode, nextVNode, optimized) {
  const { props: prevProps, children: prevChildren, component } = prevVNode;
  const { props: nextProps, children: nextChildren, patchFlag } = nextVNode;
  const emits = component.emitsOptions;
  if (false) {}
  if (nextVNode.dirs || nextVNode.transition) {
    return true;
  }
  if (optimized && patchFlag >= 0) {
    if (patchFlag & 1024) {
      return true;
    }
    if (patchFlag & 16) {
      if (!prevProps) {
        return !!nextProps;
      }
      return hasPropsChanged(prevProps, nextProps, emits);
    } else if (patchFlag & 8) {
      const dynamicProps = nextVNode.dynamicProps;
      for (let i = 0;i < dynamicProps.length; i++) {
        const key = dynamicProps[i];
        if (hasPropValueChanged(nextProps, prevProps, key) && !isEmitListener(emits, key)) {
          return true;
        }
      }
    }
  } else {
    if (prevChildren || nextChildren) {
      if (!nextChildren || !nextChildren.$stable) {
        return true;
      }
    }
    if (prevProps === nextProps) {
      return false;
    }
    if (!prevProps) {
      return !!nextProps;
    }
    if (!nextProps) {
      return true;
    }
    return hasPropsChanged(prevProps, nextProps, emits);
  }
  return false;
}
function hasPropsChanged(prevProps, nextProps, emitsOptions) {
  const nextKeys = Object.keys(nextProps);
  if (nextKeys.length !== Object.keys(prevProps).length) {
    return true;
  }
  for (let i = 0;i < nextKeys.length; i++) {
    const key = nextKeys[i];
    if (hasPropValueChanged(nextProps, prevProps, key) && !isEmitListener(emitsOptions, key)) {
      return true;
    }
  }
  return false;
}
function hasPropValueChanged(nextProps, prevProps, key) {
  const nextProp = nextProps[key];
  const prevProp = prevProps[key];
  if (key === "style" && isObject(nextProp) && isObject(prevProp)) {
    return !looseEqual(nextProp, prevProp);
  }
  return nextProp !== prevProp;
}
function updateHOCHostEl({ vnode, parent, suspense }, el) {
  while (parent) {
    const root = parent.subTree;
    if (root.suspense && root.suspense.activeBranch === vnode) {
      root.suspense.vnode.el = root.el = el;
      vnode = root;
    }
    if (root === vnode) {
      (vnode = parent.vnode).el = el;
      parent = parent.parent;
    } else {
      break;
    }
  }
  if (suspense && suspense.activeBranch === vnode) {
    suspense.vnode.el = el;
  }
}
var internalObjectProto = {};
var createInternalObject = () => Object.create(internalObjectProto);
var isInternalObject = (obj) => Object.getPrototypeOf(obj) === internalObjectProto;
function initProps(instance, rawProps, isStateful, isSSR = false) {
  const props = {};
  const attrs = createInternalObject();
  instance.propsDefaults = /* @__PURE__ */ Object.create(null);
  setFullProps(instance, rawProps, props, attrs);
  for (const key in instance.propsOptions[0]) {
    if (!(key in props)) {
      props[key] = undefined;
    }
  }
  if (false) {}
  if (isStateful) {
    instance.props = isSSR ? props : shallowReactive(props);
  } else {
    if (!instance.type.props) {
      instance.props = attrs;
    } else {
      instance.props = props;
    }
  }
  instance.attrs = attrs;
}
function updateProps(instance, rawProps, rawPrevProps, optimized) {
  const {
    props,
    attrs,
    vnode: { patchFlag }
  } = instance;
  const rawCurrentProps = toRaw(props);
  const [options] = instance.propsOptions;
  let hasAttrsChanged = false;
  if ((optimized || patchFlag > 0) && !(patchFlag & 16)) {
    if (patchFlag & 8) {
      const propsToUpdate = instance.vnode.dynamicProps;
      for (let i = 0;i < propsToUpdate.length; i++) {
        let key = propsToUpdate[i];
        if (isEmitListener(instance.emitsOptions, key)) {
          continue;
        }
        const value = rawProps[key];
        if (options) {
          if (hasOwn(attrs, key)) {
            if (value !== attrs[key]) {
              attrs[key] = value;
              hasAttrsChanged = true;
            }
          } else {
            const camelizedKey = camelize(key);
            props[camelizedKey] = resolvePropValue(options, rawCurrentProps, camelizedKey, value, instance, false);
          }
        } else {
          if (value !== attrs[key]) {
            attrs[key] = value;
            hasAttrsChanged = true;
          }
        }
      }
    }
  } else {
    if (setFullProps(instance, rawProps, props, attrs)) {
      hasAttrsChanged = true;
    }
    let kebabKey;
    for (const key in rawCurrentProps) {
      if (!rawProps || !hasOwn(rawProps, key) && ((kebabKey = hyphenate(key)) === key || !hasOwn(rawProps, kebabKey))) {
        if (options) {
          if (rawPrevProps && (rawPrevProps[key] !== undefined || rawPrevProps[kebabKey] !== undefined)) {
            props[key] = resolvePropValue(options, rawCurrentProps, key, undefined, instance, true);
          }
        } else {
          delete props[key];
        }
      }
    }
    if (attrs !== rawCurrentProps) {
      for (const key in attrs) {
        if (!rawProps || !hasOwn(rawProps, key) && true) {
          delete attrs[key];
          hasAttrsChanged = true;
        }
      }
    }
  }
  if (hasAttrsChanged) {
    trigger(instance.attrs, "set", "");
  }
  if (false) {}
}
function setFullProps(instance, rawProps, props, attrs) {
  const [options, needCastKeys] = instance.propsOptions;
  let hasAttrsChanged = false;
  let rawCastValues;
  if (rawProps) {
    for (let key in rawProps) {
      if (isReservedProp(key)) {
        continue;
      }
      const value = rawProps[key];
      let camelKey;
      if (options && hasOwn(options, camelKey = camelize(key))) {
        if (!needCastKeys || !needCastKeys.includes(camelKey)) {
          props[camelKey] = value;
        } else {
          (rawCastValues || (rawCastValues = {}))[camelKey] = value;
        }
      } else if (!isEmitListener(instance.emitsOptions, key)) {
        if (!(key in attrs) || value !== attrs[key]) {
          attrs[key] = value;
          hasAttrsChanged = true;
        }
      }
    }
  }
  if (needCastKeys) {
    const rawCurrentProps = toRaw(props);
    const castValues = rawCastValues || EMPTY_OBJ;
    for (let i = 0;i < needCastKeys.length; i++) {
      const key = needCastKeys[i];
      props[key] = resolvePropValue(options, rawCurrentProps, key, castValues[key], instance, !hasOwn(castValues, key));
    }
  }
  return hasAttrsChanged;
}
function resolvePropValue(options, props, key, value, instance, isAbsent) {
  const opt = options[key];
  if (opt != null) {
    const hasDefault = hasOwn(opt, "default");
    if (hasDefault && value === undefined) {
      const defaultValue = opt.default;
      if (opt.type !== Function && !opt.skipFactory && isFunction(defaultValue)) {
        const { propsDefaults } = instance;
        if (key in propsDefaults) {
          value = propsDefaults[key];
        } else {
          const reset = setCurrentInstance(instance);
          value = propsDefaults[key] = defaultValue.call(null, props);
          reset();
        }
      } else {
        value = defaultValue;
      }
      if (instance.ce) {
        instance.ce._setProp(key, value);
      }
    }
    if (opt[0]) {
      if (isAbsent && !hasDefault) {
        value = false;
      } else if (opt[1] && (value === "" || value === hyphenate(key))) {
        value = true;
      }
    }
  }
  return value;
}
function normalizePropsOptions(comp, appContext, asMixin = false) {
  const cache = appContext.propsCache;
  const cached = cache.get(comp);
  if (cached) {
    return cached;
  }
  const raw = comp.props;
  const normalized = {};
  const needCastKeys = [];
  let hasExtends = false;
  if (false) {}
  if (!raw && !hasExtends) {
    if (isObject(comp)) {
      cache.set(comp, EMPTY_ARR);
    }
    return EMPTY_ARR;
  }
  if (isArray(raw)) {
    for (let i = 0;i < raw.length; i++) {
      if (false) {}
      const normalizedKey = camelize(raw[i]);
      if (validatePropName(normalizedKey)) {
        normalized[normalizedKey] = EMPTY_OBJ;
      }
    }
  } else if (raw) {
    if (false) {}
    for (const key in raw) {
      const normalizedKey = camelize(key);
      if (validatePropName(normalizedKey)) {
        const opt = raw[key];
        const prop = normalized[normalizedKey] = isArray(opt) || isFunction(opt) ? { type: opt } : extend({}, opt);
        const propType = prop.type;
        let shouldCast = false;
        let shouldCastTrue = true;
        if (isArray(propType)) {
          for (let index = 0;index < propType.length; ++index) {
            const type = propType[index];
            const typeName = isFunction(type) && type.name;
            if (typeName === "Boolean") {
              shouldCast = true;
              break;
            } else if (typeName === "String") {
              shouldCastTrue = false;
            }
          }
        } else {
          shouldCast = isFunction(propType) && propType.name === "Boolean";
        }
        prop[0] = shouldCast;
        prop[1] = shouldCastTrue;
        if (shouldCast || hasOwn(prop, "default")) {
          needCastKeys.push(normalizedKey);
        }
      }
    }
  }
  const res = [normalized, needCastKeys];
  if (isObject(comp)) {
    cache.set(comp, res);
  }
  return res;
}
function validatePropName(key) {
  if (key[0] !== "$" && !isReservedProp(key)) {
    return true;
  } else if (false) {}
  return false;
}
var isInternalKey = (key) => key === "_" || key === "_ctx" || key === "$stable";
var normalizeSlotValue = (value) => isArray(value) ? value.map(normalizeVNode) : [normalizeVNode(value)];
var normalizeSlot = (key, rawSlot, ctx) => {
  if (rawSlot._n) {
    return rawSlot;
  }
  const normalized = withCtx((...args) => {
    if (false) {}
    return normalizeSlotValue(rawSlot(...args));
  }, ctx);
  normalized._c = false;
  return normalized;
};
var normalizeObjectSlots = (rawSlots, slots, instance) => {
  const ctx = rawSlots._ctx;
  for (const key in rawSlots) {
    if (isInternalKey(key))
      continue;
    const value = rawSlots[key];
    if (isFunction(value)) {
      slots[key] = normalizeSlot(key, value, ctx);
    } else if (value != null) {
      if (false) {}
      const normalized = normalizeSlotValue(value);
      slots[key] = () => normalized;
    }
  }
};
var normalizeVNodeSlots = (instance, children) => {
  if (false) {}
  const normalized = normalizeSlotValue(children);
  instance.slots.default = () => normalized;
};
var assignSlots = (slots, children, optimized) => {
  for (const key in children) {
    if (optimized || !isInternalKey(key)) {
      slots[key] = children[key];
    }
  }
};
var initSlots = (instance, children, optimized) => {
  const slots = instance.slots = createInternalObject();
  if (instance.vnode.shapeFlag & 32) {
    const type = children._;
    if (type) {
      assignSlots(slots, children, optimized);
      if (optimized) {
        def(slots, "_", type, true);
      }
    } else {
      normalizeObjectSlots(children, slots);
    }
  } else if (children) {
    normalizeVNodeSlots(instance, children);
  }
};
var updateSlots = (instance, children, optimized) => {
  const { vnode, slots } = instance;
  let needDeletionCheck = true;
  let deletionComparisonTarget = EMPTY_OBJ;
  if (vnode.shapeFlag & 32) {
    const type = children._;
    if (type) {
      if (false) {} else if (optimized && type === 1) {
        needDeletionCheck = false;
      } else {
        assignSlots(slots, children, optimized);
      }
    } else {
      needDeletionCheck = !children.$stable;
      normalizeObjectSlots(children, slots);
    }
    deletionComparisonTarget = children;
  } else if (children) {
    normalizeVNodeSlots(instance, children);
    deletionComparisonTarget = { default: 1 };
  }
  if (needDeletionCheck) {
    for (const key in slots) {
      if (!isInternalKey(key) && deletionComparisonTarget[key] == null) {
        delete slots[key];
      }
    }
  }
};
function initFeatureFlags() {
  const needWarn = [];
  if (false) {}
  if (false) {}
  if (false) {}
  if (false) {}
}
var queuePostRenderEffect = queueEffectWithSuspense;
function createRenderer(options) {
  return baseCreateRenderer(options);
}
function baseCreateRenderer(options, createHydrationFns) {
  {
    initFeatureFlags();
  }
  const target = getGlobalThis();
  target.__VUE__ = true;
  if (false) {}
  const {
    insert: hostInsert,
    remove: hostRemove,
    patchProp: hostPatchProp,
    createElement: hostCreateElement,
    createText: hostCreateText,
    createComment: hostCreateComment,
    setText: hostSetText,
    setElementText: hostSetElementText,
    parentNode: hostParentNode,
    nextSibling: hostNextSibling,
    setScopeId: hostSetScopeId = NOOP,
    insertStaticContent: hostInsertStaticContent
  } = options;
  const patch = (n1, n2, container, anchor = null, parentComponent = null, parentSuspense = null, namespace = undefined, slotScopeIds = null, optimized = !!n2.dynamicChildren) => {
    if (n1 === n2) {
      return;
    }
    if (n1 && !isSameVNodeType(n1, n2)) {
      anchor = getNextHostNode(n1);
      unmount(n1, parentComponent, parentSuspense, true);
      n1 = null;
    }
    if (n2.patchFlag === -2) {
      optimized = false;
      n2.dynamicChildren = null;
    }
    const { type, ref: ref2, shapeFlag } = n2;
    switch (type) {
      case Text:
        processText(n1, n2, container, anchor);
        break;
      case Comment:
        processCommentNode(n1, n2, container, anchor);
        break;
      case Static:
        if (n1 == null) {
          mountStaticNode(n2, container, anchor, namespace);
        } else if (false) {}
        break;
      case Fragment:
        processFragment(n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        break;
      default:
        if (shapeFlag & 1) {
          processElement(n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        } else if (shapeFlag & 6) {
          processComponent(n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        } else if (shapeFlag & 64) {
          type.process(n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized, internals);
        } else if (shapeFlag & 128) {
          type.process(n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized, internals);
        } else if (false) {}
    }
    if (ref2 != null && parentComponent) {
      setRef(ref2, n1 && n1.ref, parentSuspense, n2 || n1, !n2);
    } else if (ref2 == null && n1 && n1.ref != null) {
      setRef(n1.ref, null, parentSuspense, n1, true);
    }
  };
  const processText = (n1, n2, container, anchor) => {
    if (n1 == null) {
      hostInsert(n2.el = hostCreateText(n2.children), container, anchor);
    } else {
      const el = n2.el = n1.el;
      if (n2.children !== n1.children) {
        hostSetText(el, n2.children);
      }
    }
  };
  const processCommentNode = (n1, n2, container, anchor) => {
    if (n1 == null) {
      hostInsert(n2.el = hostCreateComment(n2.children || ""), container, anchor);
    } else {
      n2.el = n1.el;
    }
  };
  const mountStaticNode = (n2, container, anchor, namespace) => {
    [n2.el, n2.anchor] = hostInsertStaticContent(n2.children, container, anchor, namespace, n2.el, n2.anchor);
  };
  const patchStaticNode = (n1, n2, container, namespace) => {
    if (n2.children !== n1.children) {
      const anchor = hostNextSibling(n1.anchor);
      removeStaticNode(n1);
      [n2.el, n2.anchor] = hostInsertStaticContent(n2.children, container, anchor, namespace);
    } else {
      n2.el = n1.el;
      n2.anchor = n1.anchor;
    }
  };
  const moveStaticNode = ({ el, anchor }, container, nextSibling) => {
    let next;
    while (el && el !== anchor) {
      next = hostNextSibling(el);
      hostInsert(el, container, nextSibling);
      el = next;
    }
    hostInsert(anchor, container, nextSibling);
  };
  const removeStaticNode = ({ el, anchor }) => {
    let next;
    while (el && el !== anchor) {
      next = hostNextSibling(el);
      hostRemove(el);
      el = next;
    }
    hostRemove(anchor);
  };
  const processElement = (n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    if (n2.type === "svg") {
      namespace = "svg";
    } else if (n2.type === "math") {
      namespace = "mathml";
    }
    if (n1 == null) {
      mountElement(n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
    } else {
      const customElement = n1.el && n1.el._isVueCE ? n1.el : null;
      try {
        if (customElement) {
          customElement._beginPatch();
        }
        patchElement(n1, n2, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
      } finally {
        if (customElement) {
          customElement._endPatch();
        }
      }
    }
  };
  const mountElement = (vnode, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    let el;
    let vnodeHook;
    const { props, shapeFlag, transition, dirs } = vnode;
    el = vnode.el = hostCreateElement(vnode.type, namespace, props && props.is, props);
    if (shapeFlag & 8) {
      hostSetElementText(el, vnode.children);
    } else if (shapeFlag & 16) {
      mountChildren(vnode.children, el, null, parentComponent, parentSuspense, resolveChildrenNamespace(vnode, namespace), slotScopeIds, optimized);
    }
    if (dirs) {
      invokeDirectiveHook(vnode, null, parentComponent, "created");
    }
    setScopeId(el, vnode, vnode.scopeId, slotScopeIds, parentComponent);
    if (props) {
      for (const key in props) {
        if (key !== "value" && !isReservedProp(key)) {
          hostPatchProp(el, key, null, props[key], namespace, parentComponent);
        }
      }
      if ("value" in props) {
        hostPatchProp(el, "value", null, props.value, namespace);
      }
      if (vnodeHook = props.onVnodeBeforeMount) {
        invokeVNodeHook(vnodeHook, parentComponent, vnode);
      }
    }
    if (false) {}
    if (dirs) {
      invokeDirectiveHook(vnode, null, parentComponent, "beforeMount");
    }
    const needCallTransitionHooks = needTransition(parentSuspense, transition);
    if (needCallTransitionHooks) {
      transition.beforeEnter(el);
    }
    hostInsert(el, container, anchor);
    if ((vnodeHook = props && props.onVnodeMounted) || needCallTransitionHooks || dirs) {
      const isHmr = false;
      queuePostRenderEffect(() => {
        let prev;
        if (false)
          ;
        try {
          vnodeHook && invokeVNodeHook(vnodeHook, parentComponent, vnode);
          needCallTransitionHooks && transition.enter(el);
          dirs && invokeDirectiveHook(vnode, null, parentComponent, "mounted");
        } finally {
          if (false)
            ;
        }
      }, parentSuspense);
    }
  };
  const setScopeId = (el, vnode, scopeId, slotScopeIds, parentComponent) => {
    if (scopeId) {
      hostSetScopeId(el, scopeId);
    }
    if (slotScopeIds) {
      for (let i = 0;i < slotScopeIds.length; i++) {
        hostSetScopeId(el, slotScopeIds[i]);
      }
    }
    if (parentComponent) {
      let subTree = parentComponent.subTree;
      if (false) {}
      if (vnode === subTree || isSuspense(subTree.type) && (subTree.ssContent === vnode || subTree.ssFallback === vnode)) {
        const parentVNode = parentComponent.vnode;
        setScopeId(el, parentVNode, parentVNode.scopeId, parentVNode.slotScopeIds, parentComponent.parent);
      }
    }
  };
  const mountChildren = (children, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized, start = 0) => {
    for (let i = start;i < children.length; i++) {
      const child = children[i] = optimized ? cloneIfMounted(children[i]) : normalizeVNode(children[i]);
      patch(null, child, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
    }
  };
  const patchElement = (n1, n2, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    const el = n2.el = n1.el;
    if (false) {}
    let { patchFlag, dynamicChildren, dirs } = n2;
    patchFlag |= n1.patchFlag & 16;
    const oldProps = n1.props || EMPTY_OBJ;
    const newProps = n2.props || EMPTY_OBJ;
    let vnodeHook;
    parentComponent && toggleRecurse(parentComponent, false);
    if (vnodeHook = newProps.onVnodeBeforeUpdate) {
      invokeVNodeHook(vnodeHook, parentComponent, n2, n1);
    }
    if (dirs) {
      invokeDirectiveHook(n2, n1, parentComponent, "beforeUpdate");
    }
    parentComponent && toggleRecurse(parentComponent, true);
    if (dynamicChildren && (!n1.dynamicChildren || n1.dynamicChildren.length !== dynamicChildren.length)) {
      patchFlag = 0;
      optimized = false;
      dynamicChildren = null;
    }
    if (oldProps.innerHTML && newProps.innerHTML == null || oldProps.textContent && newProps.textContent == null) {
      hostSetElementText(el, "");
    }
    if (dynamicChildren) {
      patchBlockChildren(n1.dynamicChildren, dynamicChildren, el, parentComponent, parentSuspense, resolveChildrenNamespace(n2, namespace), slotScopeIds);
      if (false) {}
    } else if (!optimized) {
      patchChildren(n1, n2, el, null, parentComponent, parentSuspense, resolveChildrenNamespace(n2, namespace), slotScopeIds, false);
    }
    if (patchFlag > 0) {
      if (patchFlag & 16) {
        patchProps(el, oldProps, newProps, parentComponent, namespace);
      } else {
        if (patchFlag & 2) {
          if (oldProps.class !== newProps.class) {
            hostPatchProp(el, "class", null, newProps.class, namespace);
          }
        }
        if (patchFlag & 4) {
          hostPatchProp(el, "style", oldProps.style, newProps.style, namespace);
        }
        if (patchFlag & 8) {
          const propsToUpdate = n2.dynamicProps;
          for (let i = 0;i < propsToUpdate.length; i++) {
            const key = propsToUpdate[i];
            const prev = oldProps[key];
            const next = newProps[key];
            if (next !== prev || key === "value") {
              hostPatchProp(el, key, prev, next, namespace, parentComponent);
            }
          }
        }
      }
      if (patchFlag & 1) {
        if (n1.children !== n2.children) {
          hostSetElementText(el, n2.children);
        }
      }
    } else if (!optimized && dynamicChildren == null) {
      patchProps(el, oldProps, newProps, parentComponent, namespace);
    }
    if ((vnodeHook = newProps.onVnodeUpdated) || dirs) {
      queuePostRenderEffect(() => {
        vnodeHook && invokeVNodeHook(vnodeHook, parentComponent, n2, n1);
        dirs && invokeDirectiveHook(n2, n1, parentComponent, "updated");
      }, parentSuspense);
    }
  };
  const patchBlockChildren = (oldChildren, newChildren, fallbackContainer, parentComponent, parentSuspense, namespace, slotScopeIds) => {
    for (let i = 0;i < newChildren.length; i++) {
      const oldVNode = oldChildren[i];
      const newVNode = newChildren[i];
      const container = oldVNode.el && (oldVNode.type === Fragment || !isSameVNodeType(oldVNode, newVNode) || oldVNode.shapeFlag & (6 | 64 | 128)) ? hostParentNode(oldVNode.el) : fallbackContainer;
      patch(oldVNode, newVNode, container, null, parentComponent, parentSuspense, namespace, slotScopeIds, true);
    }
  };
  const patchProps = (el, oldProps, newProps, parentComponent, namespace) => {
    if (oldProps !== newProps) {
      if (oldProps !== EMPTY_OBJ) {
        for (const key in oldProps) {
          if (!isReservedProp(key) && !(key in newProps)) {
            hostPatchProp(el, key, oldProps[key], null, namespace, parentComponent);
          }
        }
      }
      for (const key in newProps) {
        if (isReservedProp(key))
          continue;
        const next = newProps[key];
        const prev = oldProps[key];
        if (next !== prev && key !== "value") {
          hostPatchProp(el, key, prev, next, namespace, parentComponent);
        }
      }
      if ("value" in newProps) {
        hostPatchProp(el, "value", oldProps.value, newProps.value, namespace);
      }
    }
  };
  const processFragment = (n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    const fragmentStartAnchor = n2.el = n1 ? n1.el : hostCreateText("");
    const fragmentEndAnchor = n2.anchor = n1 ? n1.anchor : hostCreateText("");
    let { patchFlag, dynamicChildren, slotScopeIds: fragmentSlotScopeIds } = n2;
    if (false) {}
    if (fragmentSlotScopeIds) {
      slotScopeIds = slotScopeIds ? slotScopeIds.concat(fragmentSlotScopeIds) : fragmentSlotScopeIds;
    }
    if (n1 == null) {
      hostInsert(fragmentStartAnchor, container, anchor);
      hostInsert(fragmentEndAnchor, container, anchor);
      mountChildren(n2.children || [], container, fragmentEndAnchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
    } else {
      if (patchFlag > 0 && patchFlag & 64 && dynamicChildren && n1.dynamicChildren && n1.dynamicChildren.length === dynamicChildren.length) {
        patchBlockChildren(n1.dynamicChildren, dynamicChildren, container, parentComponent, parentSuspense, namespace, slotScopeIds);
        if (false) {} else if (n2.key != null || parentComponent && n2 === parentComponent.subTree) {
          traverseStaticChildren(n1, n2, true);
        }
      } else {
        patchChildren(n1, n2, container, fragmentEndAnchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
      }
    }
  };
  const processComponent = (n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    n2.slotScopeIds = slotScopeIds;
    if (n1 == null) {
      if (n2.shapeFlag & 512) {
        parentComponent.ctx.activate(n2, container, anchor, namespace, optimized);
      } else {
        mountComponent(n2, container, anchor, parentComponent, parentSuspense, namespace, optimized);
      }
    } else {
      updateComponent(n1, n2, optimized);
    }
  };
  const mountComponent = (initialVNode, container, anchor, parentComponent, parentSuspense, namespace, optimized) => {
    const instance = initialVNode.component = createComponentInstance(initialVNode, parentComponent, parentSuspense);
    if (false) {}
    if (false) {}
    if (isKeepAlive(initialVNode)) {
      instance.ctx.renderer = internals;
    }
    {
      if (false) {}
      setupComponent(instance, false, optimized);
      if (false) {}
    }
    if (false)
      ;
    if (instance.asyncDep) {
      parentSuspense && parentSuspense.registerDep(instance, setupRenderEffect, optimized);
      if (!initialVNode.el) {
        const placeholder = instance.subTree = createVNode(Comment);
        processCommentNode(null, placeholder, container, anchor);
        initialVNode.placeholder = placeholder.el;
      }
    } else {
      setupRenderEffect(instance, initialVNode, container, anchor, parentSuspense, namespace, optimized);
    }
    if (false) {}
  };
  const updateComponent = (n1, n2, optimized) => {
    const instance = n2.component = n1.component;
    if (shouldUpdateComponent(n1, n2, optimized)) {
      if (instance.asyncDep && !instance.asyncResolved) {
        if (false) {}
        updateComponentPreRender(instance, n2, optimized);
        if (false) {}
        return;
      } else {
        instance.next = n2;
        instance.update();
      }
    } else {
      n2.el = n1.el;
      instance.vnode = n2;
    }
  };
  const setupRenderEffect = (instance, initialVNode, container, anchor, parentSuspense, namespace, optimized) => {
    const componentUpdateFn = () => {
      if (!instance.isMounted) {
        let vnodeHook;
        const { el, props } = initialVNode;
        const { bm, m, parent, root, type } = instance;
        const isAsyncWrapperVNode = isAsyncWrapper(initialVNode);
        toggleRecurse(instance, false);
        if (bm) {
          invokeArrayFns(bm);
        }
        if (!isAsyncWrapperVNode && (vnodeHook = props && props.onVnodeBeforeMount)) {
          invokeVNodeHook(vnodeHook, parent, initialVNode);
        }
        toggleRecurse(instance, true);
        if (el && hydrateNode) {
          const hydrateSubTree = () => {
            if (false) {}
            instance.subTree = renderComponentRoot(instance);
            if (false) {}
            if (false) {}
            hydrateNode(el, instance.subTree, instance, parentSuspense, null);
            if (false) {}
          };
          if (isAsyncWrapperVNode && type.__asyncHydrate) {
            type.__asyncHydrate(el, instance, hydrateSubTree);
          } else {
            hydrateSubTree();
          }
        } else {
          if (root.ce && root.ce._hasShadowRoot()) {
            root.ce._injectChildStyle(type, instance.parent ? instance.parent.type : undefined);
          }
          if (false) {}
          const subTree = instance.subTree = renderComponentRoot(instance);
          if (false) {}
          if (false) {}
          patch(null, subTree, container, anchor, instance, parentSuspense, namespace);
          if (false) {}
          initialVNode.el = subTree.el;
        }
        if (m) {
          queuePostRenderEffect(m, parentSuspense);
        }
        if (!isAsyncWrapperVNode && (vnodeHook = props && props.onVnodeMounted)) {
          const scopedInitialVNode = initialVNode;
          queuePostRenderEffect(() => invokeVNodeHook(vnodeHook, parent, scopedInitialVNode), parentSuspense);
        }
        if (initialVNode.shapeFlag & 256 || parent && isAsyncWrapper(parent.vnode) && parent.vnode.shapeFlag & 256) {
          instance.a && queuePostRenderEffect(instance.a, parentSuspense);
        }
        instance.isMounted = true;
        if (false) {}
        initialVNode = container = anchor = null;
      } else {
        let { next, bu, u, parent, vnode } = instance;
        {
          const nonHydratedAsyncRoot = locateNonHydratedAsyncRoot(instance);
          if (nonHydratedAsyncRoot) {
            if (next) {
              next.el = vnode.el;
              updateComponentPreRender(instance, next, optimized);
            }
            nonHydratedAsyncRoot.asyncDep.then(() => {
              queuePostRenderEffect(() => {
                if (!instance.isUnmounted)
                  update();
              }, parentSuspense);
            });
            return;
          }
        }
        let originNext = next;
        let vnodeHook;
        if (false) {}
        toggleRecurse(instance, false);
        if (next) {
          next.el = vnode.el;
          updateComponentPreRender(instance, next, optimized);
        } else {
          next = vnode;
        }
        if (bu) {
          invokeArrayFns(bu);
        }
        if (vnodeHook = next.props && next.props.onVnodeBeforeUpdate) {
          invokeVNodeHook(vnodeHook, parent, next, vnode);
        }
        toggleRecurse(instance, true);
        if (false) {}
        const nextTree = renderComponentRoot(instance);
        if (false) {}
        const prevTree = instance.subTree;
        instance.subTree = nextTree;
        if (false) {}
        patch(prevTree, nextTree, hostParentNode(prevTree.el), getNextHostNode(prevTree), instance, parentSuspense, namespace);
        if (false) {}
        next.el = nextTree.el;
        if (originNext === null) {
          updateHOCHostEl(instance, nextTree.el);
        }
        if (u) {
          queuePostRenderEffect(u, parentSuspense);
        }
        if (vnodeHook = next.props && next.props.onVnodeUpdated) {
          queuePostRenderEffect(() => invokeVNodeHook(vnodeHook, parent, next, vnode), parentSuspense);
        }
        if (false) {}
        if (false) {}
      }
    };
    instance.scope.on();
    const effect2 = instance.effect = new ReactiveEffect(componentUpdateFn);
    instance.scope.off();
    const update = instance.update = effect2.run.bind(effect2);
    const job = instance.job = effect2.runIfDirty.bind(effect2);
    job.i = instance;
    job.id = instance.uid;
    effect2.scheduler = () => queueJob(job);
    toggleRecurse(instance, true);
    if (false) {}
    update();
  };
  const updateComponentPreRender = (instance, nextVNode, optimized) => {
    nextVNode.component = instance;
    const prevProps = instance.vnode.props;
    instance.vnode = nextVNode;
    instance.next = null;
    updateProps(instance, nextVNode.props, prevProps, optimized);
    updateSlots(instance, nextVNode.children, optimized);
    pauseTracking();
    flushPreFlushCbs(instance);
    resetTracking();
  };
  const patchChildren = (n1, n2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized = false) => {
    const c1 = n1 && n1.children;
    const prevShapeFlag = n1 ? n1.shapeFlag : 0;
    const c2 = n2.children;
    const { patchFlag, shapeFlag } = n2;
    if (patchFlag > 0) {
      if (patchFlag & 128) {
        patchKeyedChildren(c1, c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        return;
      } else if (patchFlag & 256) {
        patchUnkeyedChildren(c1, c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        return;
      }
    }
    if (shapeFlag & 8) {
      if (prevShapeFlag & 16) {
        unmountChildren(c1, parentComponent, parentSuspense);
      }
      if (c2 !== c1) {
        hostSetElementText(container, c2);
      }
    } else {
      if (prevShapeFlag & 16) {
        if (shapeFlag & 16) {
          patchKeyedChildren(c1, c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        } else {
          unmountChildren(c1, parentComponent, parentSuspense, true);
        }
      } else {
        if (prevShapeFlag & 8) {
          hostSetElementText(container, "");
        }
        if (shapeFlag & 16) {
          mountChildren(c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        }
      }
    }
  };
  const patchUnkeyedChildren = (c1, c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    c1 = c1 || EMPTY_ARR;
    c2 = c2 || EMPTY_ARR;
    const oldLength = c1.length;
    const newLength = c2.length;
    const commonLength = Math.min(oldLength, newLength);
    let i;
    for (i = 0;i < commonLength; i++) {
      const nextChild = c2[i] = optimized ? cloneIfMounted(c2[i]) : normalizeVNode(c2[i]);
      patch(c1[i], nextChild, container, null, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
    }
    if (oldLength > newLength) {
      unmountChildren(c1, parentComponent, parentSuspense, true, false, commonLength);
    } else {
      mountChildren(c2, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized, commonLength);
    }
  };
  const patchKeyedChildren = (c1, c2, container, parentAnchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized) => {
    let i = 0;
    const l2 = c2.length;
    let e1 = c1.length - 1;
    let e2 = l2 - 1;
    while (i <= e1 && i <= e2) {
      const n1 = c1[i];
      const n2 = c2[i] = optimized ? cloneIfMounted(c2[i]) : normalizeVNode(c2[i]);
      if (isSameVNodeType(n1, n2)) {
        patch(n1, n2, container, null, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
      } else {
        break;
      }
      i++;
    }
    while (i <= e1 && i <= e2) {
      const n1 = c1[e1];
      const n2 = c2[e2] = optimized ? cloneIfMounted(c2[e2]) : normalizeVNode(c2[e2]);
      if (isSameVNodeType(n1, n2)) {
        patch(n1, n2, container, null, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
      } else {
        break;
      }
      e1--;
      e2--;
    }
    if (i > e1) {
      if (i <= e2) {
        const nextPos = e2 + 1;
        const anchor = nextPos < l2 ? c2[nextPos].el : parentAnchor;
        while (i <= e2) {
          patch(null, c2[i] = optimized ? cloneIfMounted(c2[i]) : normalizeVNode(c2[i]), container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
          i++;
        }
      }
    } else if (i > e2) {
      while (i <= e1) {
        unmount(c1[i], parentComponent, parentSuspense, true);
        i++;
      }
    } else {
      const s1 = i;
      const s2 = i;
      const keyToNewIndexMap = /* @__PURE__ */ new Map;
      for (i = s2;i <= e2; i++) {
        const nextChild = c2[i] = optimized ? cloneIfMounted(c2[i]) : normalizeVNode(c2[i]);
        if (nextChild.key != null) {
          if (false) {}
          keyToNewIndexMap.set(nextChild.key, i);
        }
      }
      let j;
      let patched = 0;
      const toBePatched = e2 - s2 + 1;
      let moved = false;
      let maxNewIndexSoFar = 0;
      const newIndexToOldIndexMap = new Array(toBePatched);
      for (i = 0;i < toBePatched; i++)
        newIndexToOldIndexMap[i] = 0;
      for (i = s1;i <= e1; i++) {
        const prevChild = c1[i];
        if (patched >= toBePatched) {
          unmount(prevChild, parentComponent, parentSuspense, true);
          continue;
        }
        let newIndex;
        if (prevChild.key != null) {
          newIndex = keyToNewIndexMap.get(prevChild.key);
        } else {
          for (j = s2;j <= e2; j++) {
            if (newIndexToOldIndexMap[j - s2] === 0 && isSameVNodeType(prevChild, c2[j])) {
              newIndex = j;
              break;
            }
          }
        }
        if (newIndex === undefined) {
          unmount(prevChild, parentComponent, parentSuspense, true);
        } else {
          newIndexToOldIndexMap[newIndex - s2] = i + 1;
          if (newIndex >= maxNewIndexSoFar) {
            maxNewIndexSoFar = newIndex;
          } else {
            moved = true;
          }
          patch(prevChild, c2[newIndex], container, null, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
          patched++;
        }
      }
      const increasingNewIndexSequence = moved ? getSequence(newIndexToOldIndexMap) : EMPTY_ARR;
      j = increasingNewIndexSequence.length - 1;
      for (i = toBePatched - 1;i >= 0; i--) {
        const nextIndex = s2 + i;
        const nextChild = c2[nextIndex];
        const anchorVNode = c2[nextIndex + 1];
        const anchor = nextIndex + 1 < l2 ? anchorVNode.el || resolveAsyncComponentPlaceholder(anchorVNode) : parentAnchor;
        if (newIndexToOldIndexMap[i] === 0) {
          patch(null, nextChild, container, anchor, parentComponent, parentSuspense, namespace, slotScopeIds, optimized);
        } else if (moved) {
          if (j < 0 || i !== increasingNewIndexSequence[j]) {
            move(nextChild, container, anchor, 2);
          } else {
            j--;
          }
        }
      }
    }
  };
  const move = (vnode, container, anchor, moveType, parentSuspense = null) => {
    const { el, type, transition, children, shapeFlag } = vnode;
    if (shapeFlag & 6) {
      move(vnode.component.subTree, container, anchor, moveType);
      return;
    }
    if (shapeFlag & 128) {
      vnode.suspense.move(container, anchor, moveType);
      return;
    }
    if (shapeFlag & 64) {
      type.move(vnode, container, anchor, internals);
      return;
    }
    if (type === Fragment) {
      hostInsert(el, container, anchor);
      for (let i = 0;i < children.length; i++) {
        move(children[i], container, anchor, moveType);
      }
      hostInsert(vnode.anchor, container, anchor);
      return;
    }
    if (type === Static) {
      moveStaticNode(vnode, container, anchor);
      return;
    }
    const needTransition2 = moveType !== 2 && shapeFlag & 1 && transition;
    if (needTransition2) {
      if (moveType === 0) {
        if (transition.persisted && !el[leaveCbKey]) {
          hostInsert(el, container, anchor);
        } else {
          transition.beforeEnter(el);
          hostInsert(el, container, anchor);
          queuePostRenderEffect(() => transition.enter(el), parentSuspense);
        }
      } else {
        const { leave, delayLeave, afterLeave } = transition;
        const remove22 = () => {
          if (vnode.ctx.isUnmounted) {
            hostRemove(el);
          } else {
            hostInsert(el, container, anchor);
          }
        };
        const performLeave = () => {
          const wasLeaving = el._isLeaving || !!el[leaveCbKey];
          if (el._isLeaving) {
            el[leaveCbKey](true);
          }
          if (transition.persisted && !wasLeaving) {
            remove22();
          } else {
            leave(el, () => {
              remove22();
              afterLeave && afterLeave();
            });
          }
        };
        if (delayLeave) {
          delayLeave(el, remove22, performLeave);
        } else {
          performLeave();
        }
      }
    } else {
      hostInsert(el, container, anchor);
    }
  };
  const unmount = (vnode, parentComponent, parentSuspense, doRemove = false, optimized = false) => {
    const {
      type,
      props,
      ref: ref2,
      children,
      dynamicChildren,
      shapeFlag,
      patchFlag,
      dirs,
      cacheIndex,
      memo
    } = vnode;
    if (patchFlag === -2) {
      optimized = false;
    }
    if (ref2 != null) {
      pauseTracking();
      setRef(ref2, null, parentSuspense, vnode, true);
      resetTracking();
    }
    if (cacheIndex != null) {
      parentComponent.renderCache[cacheIndex] = undefined;
    }
    if (shapeFlag & 256) {
      parentComponent.ctx.deactivate(vnode);
      return;
    }
    const shouldInvokeDirs = shapeFlag & 1 && dirs;
    const shouldInvokeVnodeHook = !isAsyncWrapper(vnode);
    let vnodeHook;
    if (shouldInvokeVnodeHook && (vnodeHook = props && props.onVnodeBeforeUnmount)) {
      invokeVNodeHook(vnodeHook, parentComponent, vnode);
    }
    if (shapeFlag & 6) {
      unmountComponent(vnode.component, parentSuspense, doRemove);
    } else {
      if (shapeFlag & 128) {
        vnode.suspense.unmount(parentSuspense, doRemove);
        return;
      }
      if (shouldInvokeDirs) {
        invokeDirectiveHook(vnode, null, parentComponent, "beforeUnmount");
      }
      if (shapeFlag & 64) {
        vnode.type.remove(vnode, parentComponent, parentSuspense, internals, doRemove);
      } else if (dynamicChildren && !dynamicChildren.hasOnce && (type !== Fragment || patchFlag > 0 && patchFlag & 64)) {
        unmountChildren(dynamicChildren, parentComponent, parentSuspense, false, true);
      } else if (type === Fragment && patchFlag & (128 | 256) || !optimized && shapeFlag & 16) {
        unmountChildren(children, parentComponent, parentSuspense);
      }
      if (doRemove) {
        remove2(vnode);
      }
    }
    const shouldInvalidateMemo = memo != null && cacheIndex == null;
    if (shouldInvokeVnodeHook && (vnodeHook = props && props.onVnodeUnmounted) || shouldInvokeDirs || shouldInvalidateMemo) {
      queuePostRenderEffect(() => {
        vnodeHook && invokeVNodeHook(vnodeHook, parentComponent, vnode);
        shouldInvokeDirs && invokeDirectiveHook(vnode, null, parentComponent, "unmounted");
        if (shouldInvalidateMemo) {
          vnode.el = null;
        }
      }, parentSuspense);
    }
  };
  const remove2 = (vnode) => {
    const { type, el, anchor, transition } = vnode;
    if (type === Fragment) {
      if (false) {} else {
        removeFragment(el, anchor);
      }
      return;
    }
    if (type === Static) {
      removeStaticNode(vnode);
      return;
    }
    const performRemove = () => {
      hostRemove(el);
      if (transition && !transition.persisted && transition.afterLeave) {
        transition.afterLeave();
      }
    };
    if (vnode.shapeFlag & 1 && transition && !transition.persisted) {
      const { leave, delayLeave } = transition;
      const performLeave = () => leave(el, performRemove);
      if (delayLeave) {
        delayLeave(vnode.el, performRemove, performLeave);
      } else {
        performLeave();
      }
    } else {
      performRemove();
    }
  };
  const removeFragment = (cur, end) => {
    let next;
    while (cur !== end) {
      next = hostNextSibling(cur);
      hostRemove(cur);
      cur = next;
    }
    hostRemove(end);
  };
  const unmountComponent = (instance, parentSuspense, doRemove) => {
    if (false) {}
    const { bum, scope, job, subTree, um, m, a } = instance;
    invalidateMount(m);
    invalidateMount(a);
    if (bum) {
      invokeArrayFns(bum);
    }
    scope.stop();
    if (job) {
      job.flags |= 8;
      unmount(subTree, instance, parentSuspense, doRemove);
    }
    if (um) {
      queuePostRenderEffect(um, parentSuspense);
    }
    queuePostRenderEffect(() => {
      instance.isUnmounted = true;
    }, parentSuspense);
    if (false) {}
  };
  const unmountChildren = (children, parentComponent, parentSuspense, doRemove = false, optimized = false, start = 0) => {
    for (let i = start;i < children.length; i++) {
      unmount(children[i], parentComponent, parentSuspense, doRemove, optimized);
    }
  };
  const getNextHostNode = (vnode) => {
    if (vnode.shapeFlag & 6) {
      return getNextHostNode(vnode.component.subTree);
    }
    if (vnode.shapeFlag & 128) {
      return vnode.suspense.next();
    }
    const el = hostNextSibling(vnode.anchor || vnode.el);
    const teleportEnd = el && el[TeleportEndKey];
    return teleportEnd ? hostNextSibling(teleportEnd) : el;
  };
  let isFlushing = false;
  const render = (vnode, container, namespace) => {
    let instance;
    if (vnode == null) {
      if (container._vnode) {
        unmount(container._vnode, null, null, true);
        instance = container._vnode.component;
      }
    } else {
      patch(container._vnode || null, vnode, container, null, null, null, namespace);
    }
    container._vnode = vnode;
    if (!isFlushing) {
      isFlushing = true;
      flushPreFlushCbs(instance);
      flushPostFlushCbs();
      isFlushing = false;
    }
  };
  const internals = {
    p: patch,
    um: unmount,
    m: move,
    r: remove2,
    mt: mountComponent,
    mc: mountChildren,
    pc: patchChildren,
    pbc: patchBlockChildren,
    n: getNextHostNode,
    o: options
  };
  let hydrate;
  let hydrateNode;
  if (createHydrationFns) {
    [hydrate, hydrateNode] = createHydrationFns(internals);
  }
  return {
    render,
    hydrate,
    createApp: createAppAPI(render, hydrate)
  };
}
function resolveChildrenNamespace({ type, props }, currentNamespace) {
  return currentNamespace === "svg" && type === "foreignObject" || currentNamespace === "mathml" && type === "annotation-xml" && props && props.encoding && props.encoding.includes("html") ? undefined : currentNamespace;
}
function toggleRecurse({ effect: effect2, job }, allowed) {
  if (allowed) {
    effect2.flags |= 32;
    job.flags |= 4;
  } else {
    effect2.flags &= -33;
    job.flags &= -5;
  }
}
function needTransition(parentSuspense, transition) {
  return (!parentSuspense || parentSuspense && !parentSuspense.pendingBranch) && transition && !transition.persisted;
}
function traverseStaticChildren(n1, n2, shallow = false) {
  const ch1 = n1.children;
  const ch2 = n2.children;
  if (isArray(ch1) && isArray(ch2)) {
    for (let i = 0;i < ch1.length; i++) {
      const c1 = ch1[i];
      let c2 = ch2[i];
      if (c2.shapeFlag & 1 && !c2.dynamicChildren) {
        if (c2.patchFlag <= 0 || c2.patchFlag === 32) {
          c2 = ch2[i] = cloneIfMounted(ch2[i]);
          c2.el = c1.el;
        }
        if (!shallow && c2.patchFlag !== -2)
          traverseStaticChildren(c1, c2);
      }
      if (c2.type === Text) {
        if (c2.patchFlag === -1) {
          c2 = ch2[i] = cloneIfMounted(c2);
        }
        c2.el = c1.el;
      }
      if (c2.type === Comment && !c2.el) {
        c2.el = c1.el;
      }
      if (false) {}
    }
  }
}
function getSequence(arr) {
  const p = arr.slice();
  const result = [0];
  let i, j, u, v, c;
  const len = arr.length;
  for (i = 0;i < len; i++) {
    const arrI = arr[i];
    if (arrI !== 0) {
      j = result[result.length - 1];
      if (arr[j] < arrI) {
        p[i] = j;
        result.push(i);
        continue;
      }
      u = 0;
      v = result.length - 1;
      while (u < v) {
        c = u + v >> 1;
        if (arr[result[c]] < arrI) {
          u = c + 1;
        } else {
          v = c;
        }
      }
      if (arrI < arr[result[u]]) {
        if (u > 0) {
          p[i] = result[u - 1];
        }
        result[u] = i;
      }
    }
  }
  u = result.length;
  v = result[u - 1];
  while (u-- > 0) {
    result[u] = v;
    v = p[v];
  }
  return result;
}
function locateNonHydratedAsyncRoot(instance) {
  const subComponent = instance.subTree.component;
  if (subComponent) {
    if (subComponent.asyncDep && !subComponent.asyncResolved) {
      return subComponent;
    } else {
      return locateNonHydratedAsyncRoot(subComponent);
    }
  }
}
function invalidateMount(hooks) {
  if (hooks) {
    for (let i = 0;i < hooks.length; i++)
      hooks[i].flags |= 8;
  }
}
function resolveAsyncComponentPlaceholder(anchorVnode) {
  if (anchorVnode.placeholder) {
    return anchorVnode.placeholder;
  }
  const instance = anchorVnode.component;
  if (instance) {
    return resolveAsyncComponentPlaceholder(instance.subTree);
  }
  return null;
}
var isSuspense = (type) => type.__isSuspense;
function queueEffectWithSuspense(fn, suspense) {
  if (suspense && suspense.pendingBranch) {
    if (isArray(fn)) {
      suspense.effects.push(...fn);
    } else {
      suspense.effects.push(fn);
    }
  } else {
    queuePostFlushCb(fn);
  }
}
var Fragment = /* @__PURE__ */ Symbol.for("v-fgt");
var Text = /* @__PURE__ */ Symbol.for("v-txt");
var Comment = /* @__PURE__ */ Symbol.for("v-cmt");
var Static = /* @__PURE__ */ Symbol.for("v-stc");
var blockStack = [];
var currentBlock = null;
function openBlock(disableTracking = false) {
  blockStack.push(currentBlock = disableTracking ? null : []);
}
function closeBlock() {
  blockStack.pop();
  currentBlock = blockStack[blockStack.length - 1] || null;
}
var isBlockTreeEnabled = 1;
function setBlockTracking(value, inVOnce = false) {
  isBlockTreeEnabled += value;
  if (value < 0 && currentBlock && inVOnce) {
    currentBlock.hasOnce = true;
  }
}
function setupBlock(vnode) {
  vnode.dynamicChildren = isBlockTreeEnabled > 0 ? currentBlock || EMPTY_ARR : null;
  closeBlock();
  if (isBlockTreeEnabled > 0 && currentBlock) {
    currentBlock.push(vnode);
  }
  return vnode;
}
function createElementBlock(type, props, children, patchFlag, dynamicProps, shapeFlag) {
  return setupBlock(createBaseVNode(type, props, children, patchFlag, dynamicProps, shapeFlag, true));
}
function createBlock(type, props, children, patchFlag, dynamicProps) {
  return setupBlock(createVNode(type, props, children, patchFlag, dynamicProps, true));
}
function isVNode(value) {
  return value ? value.__v_isVNode === true : false;
}
function isSameVNodeType(n1, n2) {
  if (false) {}
  return n1.type === n2.type && n1.key === n2.key;
}
var normalizeKey = ({ key }) => key != null ? key : null;
var normalizeRef = ({
  ref: ref2,
  ref_key,
  ref_for
}) => {
  if (typeof ref2 === "number") {
    ref2 = "" + ref2;
  }
  return ref2 != null ? isString(ref2) || isRef2(ref2) || isFunction(ref2) ? { i: currentRenderingInstance, r: ref2, k: ref_key, f: !!ref_for } : ref2 : null;
};
function createBaseVNode(type, props = null, children = null, patchFlag = 0, dynamicProps = null, shapeFlag = type === Fragment ? 0 : 1, isBlockNode = false, needFullChildrenNormalization = false) {
  const vnode = {
    __v_isVNode: true,
    __v_skip: true,
    type,
    props,
    key: props && normalizeKey(props),
    ref: props && normalizeRef(props),
    scopeId: currentScopeId,
    slotScopeIds: null,
    children,
    component: null,
    suspense: null,
    ssContent: null,
    ssFallback: null,
    dirs: null,
    transition: null,
    el: null,
    anchor: null,
    target: null,
    targetStart: null,
    targetAnchor: null,
    staticCount: 0,
    shapeFlag,
    patchFlag,
    dynamicProps,
    dynamicChildren: null,
    appContext: null,
    ctx: currentRenderingInstance
  };
  if (needFullChildrenNormalization) {
    normalizeChildren(vnode, children);
    if (shapeFlag & 128) {
      type.normalize(vnode);
    }
  } else if (children) {
    vnode.shapeFlag |= isString(children) ? 8 : 16;
  }
  if (false) {}
  if (false) {}
  if (isBlockTreeEnabled > 0 && !isBlockNode && currentBlock && (vnode.patchFlag > 0 || shapeFlag & 6) && vnode.patchFlag !== 32) {
    currentBlock.push(vnode);
  }
  return vnode;
}
var createVNode = _createVNode;
function _createVNode(type, props = null, children = null, patchFlag = 0, dynamicProps = null, isBlockNode = false) {
  if (!type || type === NULL_DYNAMIC_COMPONENT) {
    if (false) {}
    type = Comment;
  }
  if (isVNode(type)) {
    const cloned = cloneVNode(type, props, true);
    if (children) {
      normalizeChildren(cloned, children);
    }
    if (isBlockTreeEnabled > 0 && !isBlockNode && currentBlock) {
      if (cloned.shapeFlag & 6) {
        currentBlock[currentBlock.indexOf(type)] = cloned;
      } else {
        currentBlock.push(cloned);
      }
    }
    cloned.patchFlag = -2;
    return cloned;
  }
  if (isClassComponent(type)) {
    type = type.__vccOpts;
  }
  if (props) {
    props = guardReactiveProps(props);
    let { class: klass, style } = props;
    if (klass && !isString(klass)) {
      props.class = normalizeClass(klass);
    }
    if (isObject(style)) {
      if (isProxy(style) && !isArray(style)) {
        style = extend({}, style);
      }
      props.style = normalizeStyle(style);
    }
  }
  const shapeFlag = isString(type) ? 1 : isSuspense(type) ? 128 : isTeleport(type) ? 64 : isObject(type) ? 4 : isFunction(type) ? 2 : 0;
  if (false) {}
  return createBaseVNode(type, props, children, patchFlag, dynamicProps, shapeFlag, isBlockNode, true);
}
function guardReactiveProps(props) {
  if (!props)
    return null;
  return isProxy(props) || isInternalObject(props) ? extend({}, props) : props;
}
function cloneVNode(vnode, extraProps, mergeRef = false, cloneTransition = false) {
  const { props, ref: ref2, patchFlag, children, transition } = vnode;
  const mergedProps = extraProps ? mergeProps(props || {}, extraProps) : props;
  const cloned = {
    __v_isVNode: true,
    __v_skip: true,
    type: vnode.type,
    props: mergedProps,
    key: mergedProps && normalizeKey(mergedProps),
    ref: extraProps && extraProps.ref ? mergeRef && ref2 ? isArray(ref2) ? ref2.concat(normalizeRef(extraProps)) : [ref2, normalizeRef(extraProps)] : normalizeRef(extraProps) : ref2,
    scopeId: vnode.scopeId,
    slotScopeIds: vnode.slotScopeIds,
    children,
    target: vnode.target,
    targetStart: vnode.targetStart,
    targetAnchor: vnode.targetAnchor,
    staticCount: vnode.staticCount,
    shapeFlag: vnode.shapeFlag,
    patchFlag: extraProps && vnode.type !== Fragment ? patchFlag === -1 ? 16 : patchFlag | 16 : patchFlag,
    dynamicProps: vnode.dynamicProps,
    dynamicChildren: vnode.dynamicChildren,
    appContext: vnode.appContext,
    dirs: vnode.dirs,
    transition,
    component: vnode.component,
    suspense: vnode.suspense,
    ssContent: vnode.ssContent && cloneVNode(vnode.ssContent),
    ssFallback: vnode.ssFallback && cloneVNode(vnode.ssFallback),
    placeholder: vnode.placeholder,
    el: vnode.el,
    anchor: vnode.anchor,
    ctx: vnode.ctx,
    ce: vnode.ce
  };
  if (transition && cloneTransition) {
    setTransitionHooks(cloned, transition.clone(cloned));
  }
  return cloned;
}
function createTextVNode(text = " ", flag = 0) {
  return createVNode(Text, null, text, flag);
}
function createCommentVNode(text = "", asBlock = false) {
  return asBlock ? (openBlock(), createBlock(Comment, null, text)) : createVNode(Comment, null, text);
}
function normalizeVNode(child) {
  if (child == null || typeof child === "boolean") {
    return createVNode(Comment);
  } else if (isArray(child)) {
    return createVNode(Fragment, null, child.slice());
  } else if (isVNode(child)) {
    return cloneIfMounted(child);
  } else {
    return createVNode(Text, null, String(child));
  }
}
function cloneIfMounted(child) {
  return child.el === null && child.patchFlag !== -1 || child.memo ? child : cloneVNode(child);
}
function normalizeChildren(vnode, children) {
  let type = 0;
  const { shapeFlag } = vnode;
  if (children == null) {
    children = null;
  } else if (isArray(children)) {
    type = 16;
  } else if (typeof children === "object") {
    if (shapeFlag & (1 | 64)) {
      const slot = children.default;
      if (slot) {
        slot._c && (slot._d = false);
        normalizeChildren(vnode, slot());
        slot._c && (slot._d = true);
      }
      return;
    } else {
      type = 32;
      const slotFlag = children._;
      if (!slotFlag && !isInternalObject(children)) {
        children._ctx = currentRenderingInstance;
      } else if (slotFlag === 3 && currentRenderingInstance) {
        if (currentRenderingInstance.slots._ === 1) {
          children._ = 1;
        } else {
          children._ = 2;
          vnode.patchFlag |= 1024;
        }
      }
    }
  } else if (isFunction(children)) {
    if (shapeFlag & (1 | 64)) {
      normalizeChildren(vnode, { default: children });
      return;
    }
    children = { default: children, _ctx: currentRenderingInstance };
    type = 32;
  } else {
    children = String(children);
    if (shapeFlag & 64) {
      type = 16;
      children = [createTextVNode(children)];
    } else {
      type = 8;
    }
  }
  vnode.children = children;
  vnode.shapeFlag |= type;
}
function mergeProps(...args) {
  const ret = {};
  for (let i = 0;i < args.length; i++) {
    const toMerge = args[i];
    for (const key in toMerge) {
      if (key === "class") {
        if (ret.class !== toMerge.class) {
          ret.class = normalizeClass([ret.class, toMerge.class]);
        }
      } else if (key === "style") {
        ret.style = normalizeStyle([ret.style, toMerge.style]);
      } else if (isOn(key)) {
        const existing = ret[key];
        const incoming = toMerge[key];
        if (incoming && existing !== incoming && !(isArray(existing) && existing.includes(incoming))) {
          ret[key] = existing ? [].concat(existing, incoming) : incoming;
        } else if (incoming == null && existing == null && !isModelListener(key)) {
          ret[key] = incoming;
        }
      } else if (key !== "") {
        ret[key] = toMerge[key];
      }
    }
  }
  return ret;
}
function invokeVNodeHook(hook, instance, vnode, prevVNode = null) {
  callWithAsyncErrorHandling(hook, instance, 7, [
    vnode,
    prevVNode
  ]);
}
var emptyAppContext = createAppContext();
var uid = 0;
function createComponentInstance(vnode, parent, suspense) {
  const type = vnode.type;
  const appContext = (parent ? parent.appContext : vnode.appContext) || emptyAppContext;
  const instance = {
    uid: uid++,
    vnode,
    type,
    parent,
    appContext,
    root: null,
    next: null,
    subTree: null,
    effect: null,
    update: null,
    job: null,
    scope: new EffectScope(true),
    render: null,
    proxy: null,
    exposed: null,
    exposeProxy: null,
    withProxy: null,
    provides: parent ? parent.provides : Object.create(appContext.provides),
    ids: parent ? parent.ids : ["", 0, 0],
    accessCache: null,
    renderCache: [],
    components: null,
    directives: null,
    propsOptions: normalizePropsOptions(type, appContext),
    emitsOptions: normalizeEmitsOptions(type, appContext),
    emit: null,
    emitted: null,
    propsDefaults: EMPTY_OBJ,
    inheritAttrs: type.inheritAttrs,
    ctx: EMPTY_OBJ,
    data: EMPTY_OBJ,
    props: EMPTY_OBJ,
    attrs: EMPTY_OBJ,
    slots: EMPTY_OBJ,
    refs: EMPTY_OBJ,
    setupState: EMPTY_OBJ,
    setupContext: null,
    suspense,
    suspenseId: suspense ? suspense.pendingId : 0,
    asyncDep: null,
    asyncResolved: false,
    isMounted: false,
    isUnmounted: false,
    isDeactivated: false,
    bc: null,
    c: null,
    bm: null,
    m: null,
    bu: null,
    u: null,
    um: null,
    bum: null,
    da: null,
    a: null,
    rtg: null,
    rtc: null,
    ec: null,
    sp: null
  };
  if (false) {} else {
    instance.ctx = { _: instance };
  }
  instance.root = parent ? parent.root : instance;
  instance.emit = emit.bind(null, instance);
  if (vnode.ce) {
    vnode.ce(instance);
  }
  return instance;
}
var currentInstance = null;
var getCurrentInstance = () => currentInstance || currentRenderingInstance;
var internalSetCurrentInstance;
var setInSSRSetupState;
{
  const g = getGlobalThis();
  const registerGlobalSetter = (key, setter) => {
    let setters;
    if (!(setters = g[key]))
      setters = g[key] = [];
    setters.push(setter);
    return (v) => {
      if (setters.length > 1)
        setters.forEach((set) => set(v));
      else
        setters[0](v);
    };
  };
  internalSetCurrentInstance = registerGlobalSetter(`__VUE_INSTANCE_SETTERS__`, (v) => currentInstance = v);
  setInSSRSetupState = registerGlobalSetter(`__VUE_SSR_SETTERS__`, (v) => isInSSRComponentSetup = v);
}
var setCurrentInstance = (instance) => {
  const prev = currentInstance;
  internalSetCurrentInstance(instance);
  instance.scope.on();
  return () => {
    instance.scope.off();
    internalSetCurrentInstance(prev);
  };
};
var unsetCurrentInstance = () => {
  currentInstance && currentInstance.scope.off();
  internalSetCurrentInstance(null);
};
function isStatefulComponent(instance) {
  return instance.vnode.shapeFlag & 4;
}
var isInSSRComponentSetup = false;
function setupComponent(instance, isSSR = false, optimized = false) {
  isSSR && setInSSRSetupState(isSSR);
  const { props, children } = instance.vnode;
  const isStateful = isStatefulComponent(instance);
  initProps(instance, props, isStateful, isSSR);
  initSlots(instance, children, optimized || isSSR);
  const setupResult = isStateful ? setupStatefulComponent(instance, isSSR) : undefined;
  isSSR && setInSSRSetupState(false);
  return setupResult;
}
function setupStatefulComponent(instance, isSSR) {
  const Component = instance.type;
  if (false) {}
  instance.accessCache = /* @__PURE__ */ Object.create(null);
  instance.proxy = new Proxy(instance.ctx, PublicInstanceProxyHandlers);
  if (false) {}
  const { setup } = Component;
  if (setup) {
    pauseTracking();
    const setupContext = instance.setupContext = setup.length > 1 ? createSetupContext(instance) : null;
    const reset = setCurrentInstance(instance);
    const setupResult = callWithErrorHandling(setup, instance, 0, [
      instance.props,
      setupContext
    ]);
    const isAsyncSetup = isPromise(setupResult);
    resetTracking();
    reset();
    if ((isAsyncSetup || instance.sp) && !isAsyncWrapper(instance)) {
      markAsyncBoundary(instance);
    }
    if (isAsyncSetup) {
      setupResult.then(unsetCurrentInstance, unsetCurrentInstance);
      if (isSSR) {
        return setupResult.then((resolvedResult) => {
          setInSSRSetupState(true);
          try {
            handleSetupResult(instance, resolvedResult, isSSR);
          } finally {
            setInSSRSetupState(false);
          }
        }).catch((e) => {
          handleError(e, instance, 0);
        });
      } else {
        instance.asyncDep = setupResult;
        if (false) {}
      }
    } else {
      handleSetupResult(instance, setupResult, isSSR);
    }
  } else {
    finishComponentSetup(instance, isSSR);
  }
}
function handleSetupResult(instance, setupResult, isSSR) {
  if (isFunction(setupResult)) {
    if (instance.type.__ssrInlineRender) {
      instance.ssrRender = setupResult;
    } else {
      instance.render = setupResult;
    }
  } else if (isObject(setupResult)) {
    if (false) {}
    if (false) {}
    instance.setupState = proxyRefs(setupResult);
    if (false) {}
  } else if (false) {}
  finishComponentSetup(instance, isSSR);
}
var compile;
var installWithProxy;
function finishComponentSetup(instance, isSSR, skipOptions) {
  const Component = instance.type;
  if (!instance.render) {
    if (!isSSR && compile && !Component.render) {
      const template = Component.template || false;
      if (template) {
        if (false) {}
        const { isCustomElement, compilerOptions } = instance.appContext.config;
        const { delimiters, compilerOptions: componentCompilerOptions } = Component;
        const finalCompilerOptions = extend(extend({
          isCustomElement,
          delimiters
        }, compilerOptions), componentCompilerOptions);
        Component.render = compile(template, finalCompilerOptions);
        if (false) {}
      }
    }
    instance.render = Component.render || NOOP;
    if (installWithProxy) {
      installWithProxy(instance);
    }
  }
  if (false) {}
  if (false) {}
}
var attrsProxyHandlers = {
  get(target, key) {
    track(target, "get", "");
    return target[key];
  }
};
function createSetupContext(instance) {
  const expose = (exposed) => {
    if (false) {}
    instance.exposed = exposed || {};
  };
  if (false) {} else {
    return {
      attrs: new Proxy(instance.attrs, attrsProxyHandlers),
      slots: instance.slots,
      emit: instance.emit,
      expose
    };
  }
}
function getComponentPublicInstance(instance) {
  if (instance.exposed) {
    return instance.exposeProxy || (instance.exposeProxy = new Proxy(proxyRefs(markRaw(instance.exposed)), {
      get(target, key) {
        if (key in target) {
          return target[key];
        } else if (key in publicPropertiesMap) {
          return publicPropertiesMap[key](instance);
        }
      },
      has(target, key) {
        return key in target || key in publicPropertiesMap;
      }
    }));
  } else {
    return instance.proxy;
  }
}
function getComponentName(Component, includeInferred = true) {
  return isFunction(Component) ? Component.displayName || Component.name : Component.name || includeInferred && Component.__name;
}
function isClassComponent(value) {
  return isFunction(value) && "__vccOpts" in value;
}
var computed2 = (getterOrOptions, debugOptions) => {
  const c = computed(getterOrOptions, debugOptions, isInSSRComponentSetup);
  if (false) {}
  return c;
};
var version = "3.5.41";
// node_modules/@vue/runtime-dom/dist/runtime-dom.esm-bundler.js
var policy = undefined;
var tt = typeof window !== "undefined" && window.trustedTypes;
if (tt) {
  try {
    policy = /* @__PURE__ */ tt.createPolicy("vue", {
      createHTML: (val) => val
    });
  } catch (e) {}
}
var unsafeToTrustedHTML = policy ? (val) => policy.createHTML(val) : (val) => val;
var svgNS = "http://www.w3.org/2000/svg";
var mathmlNS = "http://www.w3.org/1998/Math/MathML";
var doc = typeof document !== "undefined" ? document : null;
var templateContainer = doc && /* @__PURE__ */ doc.createElement("template");
var nodeOps = {
  insert: (child, parent, anchor) => {
    parent.insertBefore(child, anchor || null);
  },
  remove: (child) => {
    const parent = child.parentNode;
    if (parent) {
      parent.removeChild(child);
    }
  },
  createElement: (tag, namespace, is, props) => {
    const el = namespace === "svg" ? doc.createElementNS(svgNS, tag) : namespace === "mathml" ? doc.createElementNS(mathmlNS, tag) : is ? doc.createElement(tag, { is }) : doc.createElement(tag);
    if (tag === "select" && props && props.multiple != null) {
      el.setAttribute("multiple", props.multiple);
    }
    return el;
  },
  createText: (text) => doc.createTextNode(text),
  createComment: (text) => doc.createComment(text),
  setText: (node, text) => {
    node.nodeValue = text;
  },
  setElementText: (el, text) => {
    el.textContent = text;
  },
  parentNode: (node) => node.parentNode,
  nextSibling: (node) => node.nextSibling,
  querySelector: (selector) => doc.querySelector(selector),
  setScopeId(el, id) {
    el.setAttribute(id, "");
  },
  insertStaticContent(content, parent, anchor, namespace, start, end) {
    const before = anchor ? anchor.previousSibling : parent.lastChild;
    if (start && (start === end || start.nextSibling)) {
      while (true) {
        parent.insertBefore(start.cloneNode(true), anchor);
        if (start === end || !(start = start.nextSibling))
          break;
      }
    } else {
      templateContainer.innerHTML = unsafeToTrustedHTML(namespace === "svg" ? `<svg>${content}</svg>` : namespace === "mathml" ? `<math>${content}</math>` : content);
      const template = templateContainer.content;
      if (namespace === "svg" || namespace === "mathml") {
        const wrapper = template.firstChild;
        while (wrapper.firstChild) {
          template.appendChild(wrapper.firstChild);
        }
        template.removeChild(wrapper);
      }
      parent.insertBefore(template, anchor);
    }
    return [
      before ? before.nextSibling : parent.firstChild,
      anchor ? anchor.previousSibling : parent.lastChild
    ];
  }
};
var vtcKey = /* @__PURE__ */ Symbol("_vtc");
function patchClass(el, value, isSVG) {
  const transitionClasses = el[vtcKey];
  if (transitionClasses) {
    value = (value ? [value, ...transitionClasses] : [...transitionClasses]).join(" ");
  }
  if (value == null) {
    el.removeAttribute("class");
  } else if (isSVG) {
    el.setAttribute("class", value);
  } else {
    el.className = value;
  }
}
var vShowOriginalDisplay = /* @__PURE__ */ Symbol("_vod");
var vShowHidden = /* @__PURE__ */ Symbol("_vsh");
var CSS_VAR_TEXT = /* @__PURE__ */ Symbol("");
var displayRE = /(?:^|;)\s*display\s*:/;
function patchStyle(el, prev, next) {
  const style = el.style;
  const isCssString = isString(next);
  let hasControlledDisplay = false;
  if (next && !isCssString) {
    if (prev) {
      if (!isString(prev)) {
        for (const key in prev) {
          if (next[key] == null) {
            setStyle(style, key, "");
          }
        }
      } else {
        for (const prevStyle of prev.split(";")) {
          const key = prevStyle.slice(0, prevStyle.indexOf(":")).trim();
          if (next[key] == null) {
            setStyle(style, key, "");
          }
        }
      }
    }
    for (const key in next) {
      if (key === "display") {
        hasControlledDisplay = true;
      }
      const value = next[key];
      if (value != null) {
        if (!shouldPreserveTextareaResizeStyle(el, key, !isString(prev) && prev ? prev[key] : undefined, value)) {
          setStyle(style, key, value);
        }
      } else {
        setStyle(style, key, "");
      }
    }
  } else {
    if (isCssString) {
      if (prev !== next) {
        const cssVarText = style[CSS_VAR_TEXT];
        if (cssVarText) {
          next += ";" + cssVarText;
        }
        style.cssText = next;
        hasControlledDisplay = displayRE.test(next);
      }
    } else if (prev) {
      el.removeAttribute("style");
    }
  }
  if (vShowOriginalDisplay in el) {
    el[vShowOriginalDisplay] = hasControlledDisplay ? style.display : "";
    if (el[vShowHidden]) {
      style.display = "none";
    }
  }
}
var importantRE = /\s*!important$/;
function setStyle(style, name, val) {
  if (isArray(val)) {
    val.forEach((v) => setStyle(style, name, v));
  } else {
    if (val == null)
      val = "";
    if (false) {}
    if (name.startsWith("--")) {
      style.setProperty(name, val);
    } else {
      const prefixed = autoPrefix(style, name);
      if (importantRE.test(val)) {
        style.setProperty(hyphenate(prefixed), val.replace(importantRE, ""), "important");
      } else {
        style[prefixed] = val;
      }
    }
  }
}
var prefixes = ["Webkit", "Moz", "ms"];
var prefixCache = {};
function autoPrefix(style, rawName) {
  const cached = prefixCache[rawName];
  if (cached) {
    return cached;
  }
  let name = camelize(rawName);
  if (name !== "filter" && name in style) {
    return prefixCache[rawName] = name;
  }
  name = capitalize(name);
  for (let i = 0;i < prefixes.length; i++) {
    const prefixed = prefixes[i] + name;
    if (prefixed in style) {
      return prefixCache[rawName] = prefixed;
    }
  }
  return rawName;
}
function shouldPreserveTextareaResizeStyle(el, key, prev, next) {
  return el.tagName === "TEXTAREA" && (key === "width" || key === "height") && isString(next) && prev === next;
}
var xlinkNS = "http://www.w3.org/1999/xlink";
function patchAttr(el, key, value, isSVG, instance, isBoolean = isSpecialBooleanAttr(key)) {
  if (isSVG && key.startsWith("xlink:")) {
    if (value == null) {
      el.removeAttributeNS(xlinkNS, key.slice(6, key.length));
    } else {
      el.setAttributeNS(xlinkNS, key, value);
    }
  } else {
    if (value == null || isBoolean && !includeBooleanAttr(value)) {
      el.removeAttribute(key);
    } else {
      el.setAttribute(key, isBoolean ? "" : isSymbol(value) ? String(value) : value);
    }
  }
}
function patchDOMProp(el, key, value, parentComponent, attrName) {
  if (key === "innerHTML" || key === "textContent") {
    if (value != null) {
      el[key] = key === "innerHTML" ? unsafeToTrustedHTML(value) : value;
    }
    return;
  }
  const tag = el.tagName;
  if (key === "value" && tag !== "PROGRESS" && !tag.includes("-")) {
    const oldValue = tag === "OPTION" ? el.getAttribute("value") || "" : el.value;
    const newValue = value == null ? el.type === "checkbox" ? "on" : "" : String(value);
    if (oldValue !== newValue || !("_value" in el)) {
      el.value = newValue;
    }
    if (value == null) {
      el.removeAttribute(key);
    }
    el._value = value;
    return;
  }
  let needRemove = false;
  if (value === "" || value == null) {
    const type = typeof el[key];
    if (type === "boolean") {
      value = includeBooleanAttr(value);
    } else if (value == null && type === "string") {
      value = "";
      needRemove = true;
    } else if (type === "number") {
      value = 0;
      needRemove = true;
    }
  }
  try {
    el[key] = value;
  } catch (e) {
    if (false) {}
  }
  needRemove && el.removeAttribute(attrName || key);
}
function addEventListener(el, event, handler, options) {
  el.addEventListener(event, handler, options);
}
function removeEventListener(el, event, handler, options) {
  el.removeEventListener(event, handler, options);
}
var veiKey = /* @__PURE__ */ Symbol("_vei");
function patchEvent(el, rawName, prevValue, nextValue, instance = null) {
  const invokers = el[veiKey] || (el[veiKey] = {});
  const existingInvoker = invokers[rawName];
  if (nextValue && existingInvoker) {
    existingInvoker.value = nextValue;
  } else {
    const [name, options] = parseName(rawName);
    if (nextValue) {
      const invoker = invokers[rawName] = createInvoker(nextValue, instance);
      addEventListener(el, name, invoker, options);
    } else if (existingInvoker) {
      removeEventListener(el, name, existingInvoker, options);
      invokers[rawName] = undefined;
    }
  }
}
var optionsModifierRE = /(Once|Passive|Capture)$/;
var optionsModifierEventRE = /^on:?(?:Once|Passive|Capture)$/;
function parseName(name) {
  let options;
  let m;
  while ((m = name.match(optionsModifierRE)) && !optionsModifierEventRE.test(name)) {
    if (!options)
      options = {};
    name = name.slice(0, name.length - m[1].length);
    options[m[1].toLowerCase()] = true;
  }
  const event = name[2] === ":" ? name.slice(3) : hyphenate(name.slice(2));
  return [event, options];
}
var cachedNow = 0;
var p = /* @__PURE__ */ Promise.resolve();
var getNow = () => cachedNow || (p.then(() => cachedNow = 0), cachedNow = Date.now());
function createInvoker(initialValue, instance) {
  const invoker = (e) => {
    if (!e._vts) {
      e._vts = Date.now();
    } else if (e._vts <= invoker.attached) {
      return;
    }
    const value = invoker.value;
    if (isArray(value)) {
      const originalStop = e.stopImmediatePropagation;
      e.stopImmediatePropagation = () => {
        originalStop.call(e);
        e._stopped = true;
      };
      const handlers = value.slice();
      const args = [e];
      for (let i = 0;i < handlers.length; i++) {
        if (e._stopped) {
          break;
        }
        const handler = handlers[i];
        if (handler) {
          callWithAsyncErrorHandling(handler, instance, 5, args);
        }
      }
    } else {
      callWithAsyncErrorHandling(value, instance, 5, [e]);
    }
  };
  invoker.value = initialValue;
  invoker.attached = getNow();
  return invoker;
}
var isNativeOn = (key) => key.charCodeAt(0) === 111 && key.charCodeAt(1) === 110 && key.charCodeAt(2) > 96 && key.charCodeAt(2) < 123;
var patchProp = (el, key, prevValue, nextValue, namespace, parentComponent) => {
  const isSVG = namespace === "svg";
  if (key === "class") {
    patchClass(el, nextValue, isSVG);
  } else if (key === "style") {
    patchStyle(el, prevValue, nextValue);
  } else if (isOn(key)) {
    if (!isModelListener(key)) {
      patchEvent(el, key, prevValue, nextValue, parentComponent);
    }
  } else if (key[0] === "." ? (key = key.slice(1), true) : key[0] === "^" ? (key = key.slice(1), false) : shouldSetAsProp(el, key, nextValue, isSVG)) {
    patchDOMProp(el, key, nextValue);
    if (!el.tagName.includes("-") && (key === "value" || key === "checked" || key === "selected")) {
      patchAttr(el, key, nextValue, isSVG, parentComponent, key !== "value");
    }
  } else if (el._isVueCE && (shouldSetAsPropForVueCE(el, key) || el._def.__asyncLoader && (/[A-Z]/.test(key) || !isString(nextValue)))) {
    patchDOMProp(el, camelize(key), nextValue, parentComponent, key);
  } else {
    if (key === "true-value") {
      el._trueValue = nextValue;
    } else if (key === "false-value") {
      el._falseValue = nextValue;
    }
    patchAttr(el, key, nextValue, isSVG);
  }
};
function shouldSetAsProp(el, key, value, isSVG) {
  if (isSVG) {
    if (key === "innerHTML" || key === "textContent") {
      return true;
    }
    if (key in el && isNativeOn(key) && isFunction(value)) {
      return true;
    }
    return false;
  }
  if (key === "spellcheck" || key === "draggable" || key === "translate" || key === "autocorrect") {
    return false;
  }
  if (key === "sandbox" && el.tagName === "IFRAME") {
    return false;
  }
  if (key === "form") {
    return false;
  }
  if (key === "list" && el.tagName === "INPUT") {
    return false;
  }
  if (key === "type" && el.tagName === "TEXTAREA") {
    return false;
  }
  if (key === "width" || key === "height") {
    const tag = el.tagName;
    if (tag === "IMG" || tag === "VIDEO" || tag === "CANVAS" || tag === "SOURCE") {
      return false;
    }
  }
  if (isNativeOn(key) && isString(value)) {
    return false;
  }
  return key in el;
}
function shouldSetAsPropForVueCE(el, key) {
  const props = el._def.props;
  if (!props) {
    return false;
  }
  const camelKey = camelize(key);
  return Array.isArray(props) ? props.some((prop) => camelize(prop) === camelKey) : Object.keys(props).some((prop) => camelize(prop) === camelKey);
}
var getModelAssigner = (vnode) => {
  const fn = vnode.props["onUpdate:modelValue"] || false;
  return isArray(fn) ? (value) => invokeArrayFns(fn, value) : fn;
};
function onCompositionStart(e) {
  e.target.composing = true;
}
function onCompositionEnd(e) {
  const target = e.target;
  if (target.composing) {
    target.composing = false;
    target.dispatchEvent(new Event("input"));
  }
}
var assignKey = /* @__PURE__ */ Symbol("_assign");
var initialValueKey = /* @__PURE__ */ Symbol("_initialValue");
function castValue(value, trim, number) {
  if (trim)
    value = value.trim();
  if (number)
    value = looseToNumber(value);
  return value;
}
var vModelText = {
  created(el, { modifiers: { lazy, trim, number } }, vnode) {
    if (el.parentNode) {
      if (el.type === "text") {
        el[initialValueKey] = el.defaultValue.replace(/[\r\n]/g, "");
      } else if (el.type === "textarea") {
        el[initialValueKey] = el.defaultValue.replace(/\r\n?/g, `
`);
      }
    }
    el[assignKey] = getModelAssigner(vnode);
    const castToNumber = number || vnode.props && vnode.props.type === "number";
    addEventListener(el, lazy ? "change" : "input", (e) => {
      if (e.target.composing)
        return;
      el[assignKey](castValue(el.value, trim, castToNumber));
    });
    if (trim || castToNumber) {
      addEventListener(el, "change", () => {
        el.value = castValue(el.value, trim, castToNumber);
      });
    }
    if (!lazy) {
      addEventListener(el, "compositionstart", onCompositionStart);
      addEventListener(el, "compositionend", onCompositionEnd);
      addEventListener(el, "change", onCompositionEnd);
    }
  },
  mounted(el, { value, modifiers: { trim, number } }) {
    const newValue = value == null ? "" : value;
    const initialValue = el[initialValueKey];
    delete el[initialValueKey];
    if (initialValue !== undefined && (el.type === "text" || el.type === "textarea") && el.value !== initialValue) {
      el[assignKey](castValue(el.value, trim, number));
    } else {
      el.value = newValue;
    }
  },
  beforeUpdate(el, { value, oldValue, modifiers: { lazy, trim, number } }, vnode) {
    el[assignKey] = getModelAssigner(vnode);
    if (el.composing)
      return;
    const elValue = (number || el.type === "number") && !/^0\d/.test(el.value) ? looseToNumber(el.value) : el.value;
    const newValue = value == null ? "" : value;
    if (elValue === newValue) {
      return;
    }
    const rootNode = el.getRootNode();
    if ((rootNode instanceof Document || rootNode instanceof ShadowRoot) && rootNode.activeElement === el && el.type !== "range") {
      if (lazy && value === oldValue) {
        return;
      }
      if (trim && el.value.trim() === newValue) {
        return;
      }
    }
    el.value = newValue;
  }
};
var vModelSelect = {
  deep: true,
  created(el, { value, modifiers: { number } }, vnode) {
    el._modelValue = value;
    addEventListener(el, "change", () => {
      const selectedVal = Array.prototype.filter.call(el.options, (o) => o.selected).map((o) => number ? looseToNumber(getValue(o)) : getValue(o));
      el[assignKey](el.multiple ? isSet(el._modelValue) ? new Set(selectedVal) : selectedVal : selectedVal[0]);
      el._assigning = true;
      nextTick(() => {
        el._assigning = false;
      });
    });
    el[assignKey] = getModelAssigner(vnode);
  },
  mounted(el, { value }) {
    setSelected(el, value);
  },
  beforeUpdate(el, { value }, vnode) {
    el._modelValue = value;
    el[assignKey] = getModelAssigner(vnode);
  },
  updated(el, { value }) {
    if (!el._assigning) {
      setSelected(el, value);
    }
  }
};
function setSelected(el, value) {
  const isMultiple = el.multiple;
  const isArrayValue = isArray(value);
  if (isMultiple && !isArrayValue && !isSet(value)) {
    return;
  }
  for (let i = 0, l = el.options.length;i < l; i++) {
    const option = el.options[i];
    const optionValue = getValue(option);
    if (isMultiple) {
      if (isArrayValue) {
        const optionType = typeof optionValue;
        if (optionType === "string" || optionType === "number") {
          option.selected = value.some((v) => String(v) === String(optionValue));
        } else {
          option.selected = looseIndexOf(value, optionValue) > -1;
        }
      } else {
        option.selected = value.has(optionValue);
      }
    } else if (looseEqual(getValue(option), value)) {
      if (el.selectedIndex !== i)
        el.selectedIndex = i;
      return;
    }
  }
  if (!isMultiple && el.selectedIndex !== -1) {
    el.selectedIndex = -1;
  }
}
function getValue(el) {
  return "_value" in el ? el._value : el.value;
}
var systemModifiers = ["ctrl", "shift", "alt", "meta"];
var modifierGuards = {
  stop: (e) => e.stopPropagation(),
  prevent: (e) => e.preventDefault(),
  self: (e) => e.target !== e.currentTarget,
  ctrl: (e) => !e.ctrlKey,
  shift: (e) => !e.shiftKey,
  alt: (e) => !e.altKey,
  meta: (e) => !e.metaKey,
  left: (e) => ("button" in e) && e.button !== 0,
  middle: (e) => ("button" in e) && e.button !== 1,
  right: (e) => ("button" in e) && e.button !== 2,
  exact: (e, modifiers) => systemModifiers.some((m) => e[`${m}Key`] && !modifiers.includes(m))
};
var withModifiers = (fn, modifiers) => {
  if (!fn)
    return fn;
  const cache = fn._withMods || (fn._withMods = {});
  const cacheKey = modifiers.join(".");
  return cache[cacheKey] || (cache[cacheKey] = (event, ...args) => {
    for (let i = 0;i < modifiers.length; i++) {
      const guard = modifierGuards[modifiers[i]];
      if (guard && guard(event, modifiers))
        return;
    }
    return fn(event, ...args);
  });
};
var keyNames = {
  esc: "escape",
  space: " ",
  up: "arrow-up",
  left: "arrow-left",
  right: "arrow-right",
  down: "arrow-down",
  delete: "backspace"
};
var withKeys = (fn, modifiers) => {
  const cache = fn._withKeys || (fn._withKeys = {});
  const cacheKey = modifiers.join(".");
  return cache[cacheKey] || (cache[cacheKey] = (event) => {
    if (!("key" in event)) {
      return;
    }
    const eventKey = hyphenate(event.key);
    if (modifiers.some((k) => k === eventKey || keyNames[k] === eventKey)) {
      return fn(event);
    }
  });
};
var rendererOptions = /* @__PURE__ */ extend({ patchProp }, nodeOps);
var renderer;
function ensureRenderer() {
  return renderer || (renderer = createRenderer(rendererOptions));
}
var createApp = (...args) => {
  const app = ensureRenderer().createApp(...args);
  if (false) {}
  const { mount } = app;
  app.mount = (containerOrSelector) => {
    const container = normalizeContainer(containerOrSelector);
    if (!container)
      return;
    const component = app._component;
    if (!isFunction(component) && !component.render && !component.template) {
      component.template = container.innerHTML;
    }
    if (container.nodeType === 1) {
      container.textContent = "";
    }
    const proxy = mount(container, false, resolveRootNamespace(container));
    if (container instanceof Element) {
      container.removeAttribute("v-cloak");
      container.setAttribute("data-v-app", "");
    }
    return proxy;
  };
  return app;
};
function resolveRootNamespace(container) {
  if (container instanceof SVGElement) {
    return "svg";
  }
  if (typeof MathMLElement === "function" && container instanceof MathMLElement) {
    return "mathml";
  }
}
function normalizeContainer(container) {
  if (isString(container)) {
    const res = document.querySelector(container);
    if (false) {}
    return res;
  }
  if (false) {}
  return container;
}
// node_modules/vue/dist/vue.runtime.esm-bundler.js
if (false) {}

// frontend/api.ts
class ApiError extends Error {
  status;
  constructor(status, message) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}
async function api(path, method = "GET", body) {
  const init = { method };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }
  const response = await fetch(path, init);
  if (!response.ok) {
    throw new ApiError(response.status, await errorMessage(response));
  }
  return await response.json();
}
async function errorMessage(response) {
  try {
    const payload = await response.json();
    if (payload.error)
      return payload.error;
  } catch {}
  return `${response.status} ${response.statusText}`;
}
var fetchStatus = () => api("/api/status");
var fetchConfig = () => api("/api/config");
var fetchTasks = () => api("/api/tasks");
var fetchTask = (id) => api(`/api/tasks/${id}`);
var createTask = (input) => api("/api/tasks", "POST", input);
var patchTask = (id, patch) => api(`/api/tasks/${id}`, "PATCH", patch);
var addNote = (id, text) => api(`/api/tasks/${id}/notes`, "POST", { text });
function describe(error) {
  return error instanceof Error ? error.message : String(error);
}

// frontend/events.ts
var RECONNECT_MIN_MS = 500;
var RECONNECT_MAX_MS = 15000;
function backoffDelay(attempt) {
  if (attempt <= 0)
    return RECONNECT_MIN_MS;
  const grown = RECONNECT_MIN_MS * 2 ** attempt;
  return Number.isFinite(grown) ? Math.min(grown, RECONNECT_MAX_MS) : RECONNECT_MAX_MS;
}
var SILENCE_TIMEOUT_MS = 65000;
function connectEvents(handlers, url = "/api/events") {
  let source = null;
  let timer;
  let watchdog;
  let attempt = 0;
  let stopped = false;
  const drop = () => {
    clearTimeout(watchdog);
    source?.close();
    source = null;
    handlers.onConnected(false);
    if (stopped)
      return;
    timer = setTimeout(open, backoffDelay(attempt));
    attempt += 1;
  };
  const heard = () => {
    clearTimeout(watchdog);
    watchdog = setTimeout(drop, SILENCE_TIMEOUT_MS);
  };
  const open = () => {
    if (stopped)
      return;
    source = new EventSource(url);
    heard();
    source.addEventListener("open", () => {
      attempt = 0;
      heard();
      handlers.onConnected(true);
    });
    source.addEventListener("ping", heard);
    source.addEventListener("tasks", () => {
      heard();
      handlers.onTasks();
    });
    source.addEventListener("config", () => {
      heard();
      handlers.onConfig();
    });
    source.addEventListener("scan-failed", (message) => {
      heard();
      handlers.onScanFailed(message.data);
    });
    source.addEventListener("error", drop);
  };
  open();
  return () => {
    stopped = true;
    clearTimeout(timer);
    clearTimeout(watchdog);
    source?.close();
    source = null;
  };
}

// frontend/board.ts
var FALLBACK_COLUMNS = [
  { name: "inbox", display_name: "Inbox", default: true },
  { name: "todo", display_name: "To do", consider_ready: true },
  { name: "in-progress", display_name: "In Progress" },
  { name: "done", display_name: "Done", consider_done: true },
  { name: "rejected", display_name: "Rejected" }
];
function findColumn(status, columns) {
  return columns.find((column) => column.name === status);
}
function columnDisplay(status, columns) {
  return findColumn(status, columns)?.display_name || status;
}
function columnOffersWork(status, columns) {
  return findColumn(status, columns)?.consider_ready === true;
}
function columnSatisfies(status, columns) {
  return findColumn(status, columns)?.consider_done === true;
}
function defaultColumn(columns) {
  return (columns.find((column) => column.default) ?? columns[0])?.name ?? "";
}
function indexTasks(tasks) {
  return new Map(tasks.map((task) => [task.id, task]));
}
function pendingDependencies(task, index, columns) {
  return (task.depends_on ?? []).filter((id) => {
    const other = index.get(id);
    return other === undefined || !columnSatisfies(other.status, columns);
  });
}
function isReady(task, index, columns) {
  if (!columnOffersWork(task.status, columns))
    return false;
  return pendingDependencies(task, index, columns).length === 0;
}
var LABEL_SEPARATOR = "/";
function isConfigured(name, labels) {
  return Object.hasOwn(labels, name);
}
function definitionOf(name, labels) {
  return isConfigured(name, labels) ? labels[name] : undefined;
}
var LABEL_JOINER = " | ";
function titleCase(text) {
  return text.split(LABEL_SEPARATOR).map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1)).join(LABEL_SEPARATOR);
}
function labelHalves(name, labels) {
  const configured = definitionOf(name, labels)?.display_name ?? "";
  const named = configured === name ? "" : configured;
  const at = name.indexOf(LABEL_SEPARATOR);
  const scope = at > 0 ? name.slice(0, at) : "";
  const value = at > 0 ? name.slice(at + LABEL_SEPARATOR.length) : "";
  if (scope === "" || value === "")
    return { scope: "", value: named || name };
  return { scope: titleCase(scope), value: named || titleCase(value) };
}
function labelDisplay(name, labels) {
  const { scope, value } = labelHalves(name, labels);
  return scope === "" ? value : scope + LABEL_JOINER + value;
}
function labelsInUse(tasks) {
  const names = new Set;
  for (const task of tasks)
    for (const label of task.labels ?? [])
      names.add(label);
  return [...names].sort();
}
var CHIP_DARK_TEXT = "#111418";
var CHIP_LIGHT_TEXT = "#ffffff";
var HEX_COLOR = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i;
function labelChip(name, labels) {
  const color = definitionOf(name, labels)?.color ?? "";
  if (!HEX_COLOR.test(color))
    return null;
  return { background: color, text: readableText(color) };
}
function readableText(color) {
  const background = luminance(color);
  const onDark = contrast(background, luminance(CHIP_DARK_TEXT));
  const onLight = contrast(background, luminance(CHIP_LIGHT_TEXT));
  return onDark >= onLight ? CHIP_DARK_TEXT : CHIP_LIGHT_TEXT;
}
function luminance(color) {
  const digits = color.slice(1);
  const full = digits.length === 3 ? [...digits].map((digit) => digit + digit).join("") : digits;
  const [red, green, blue] = [0, 2, 4].map((at) => {
    const value = parseInt(full.slice(at, at + 2), 16) / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}
var contrast = (a, b) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
function findPriority(name, priorities) {
  return priorities.find((priority) => priority.name === name);
}
function defaultPriority(priorities) {
  return (priorities.find((priority) => priority.default) ?? priorities[0])?.name ?? "";
}
function priorityDisplay(name, priorities) {
  return findPriority(name, priorities)?.display_name || name;
}
function priorityChip(name, priorities) {
  const color = findPriority(name, priorities)?.color ?? "";
  if (!HEX_COLOR.test(color))
    return null;
  return { background: color, text: readableText(color) };
}
function priorityOptions(priorities, extras) {
  const options = priorities.map((priority) => ({
    name: priority.name,
    display: priority.display_name || priority.name,
    configured: true
  }));
  const unknown = [...new Set(extras)].filter((name) => name !== "" && !findPriority(name, priorities)).sort();
  return [...options, ...unknown.map((name) => ({ name, display: name, configured: false }))];
}
function visibleTasks(tasks, filters, columns) {
  const { status, priority, assignee, label, ready, text, excluded } = filters;
  const index = indexTasks(tasks);
  return tasks.filter((task) => {
    if (status && !isValue(task.status, status))
      return false;
    if (priority && !isValue(task.priority ?? "", priority))
      return false;
    if (assignee && !contains(task.assignee ?? "", assignee))
      return false;
    if (label && !carriesLabel(task, label))
      return false;
    if (ready && !isReady(task, index, columns))
      return false;
    for (const term of excluded ?? []) {
      if (carriesValue(task, term))
        return false;
    }
    for (const term of text ?? []) {
      const wanted = term.value.trim().toLowerCase();
      if (wanted === "")
        continue;
      if (carriesText(task, wanted) === term.negated)
        return false;
    }
    return true;
  });
}
function isValue(held, wanted) {
  return held.toLowerCase() === wanted.trim().toLowerCase();
}
function contains(haystack, needle) {
  return haystack.toLowerCase().includes(needle.trim().toLowerCase());
}
function carriesLabel(task, wanted) {
  return (task.labels ?? []).some((label) => isValue(label, wanted));
}
function carriesValue(task, term) {
  switch (term.key) {
    case "status":
      return isValue(task.status, term.value);
    case "priority":
      return isValue(task.priority ?? "", term.value);
    case "assignee":
      return contains(task.assignee ?? "", term.value);
    case "label":
      return carriesLabel(task, term.value);
  }
}
function carriesText(task, wanted) {
  return [task.id, task.title, task.body].some((field) => (field ?? "").toLowerCase().includes(wanted));
}

// frontend/state.ts
var POLL_INTERVAL_MS = 3000;
var NOT_FOUND = 404;
var FALLBACK_PRIORITIES = [
  { name: "urgent", color: "#b42318", display_name: "Urgent" },
  { name: "high", color: "#c2410c", display_name: "High" },
  { name: "normal", color: "#4b5563", display_name: "Normal", default: true },
  { name: "low", color: "#6b7280", display_name: "Low" }
];
var tasks = ref([]);
var filters = reactive({
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
  text: [],
  excluded: []
});
var labels = ref({});
var priorities = ref(FALLBACK_PRIORITIES);
var columns = ref(FALLBACK_COLUMNS);
var taskDir = ref("");
var version2 = ref("");
var unreadable = ref([]);
var duplicated = ref([]);
var incomplete = ref(false);
var loaded = ref(false);
var streaming = ref(null);
var stale = ref(false);
var index = computed2(() => indexTasks(tasks.value));
var visible = computed2(() => visibleTasks(tasks.value, filters, columns.value));
var statusLine = computed2(() => {
  if (!loaded.value)
    return "Loading…";
  const total = tasks.value.length;
  const shown = visible.value.length;
  const counts = shown === total ? `${total} tasks` : `${shown} of ${total} tasks`;
  const link = streaming.value === false ? "polling" : "";
  const broken = unreadable.value.length;
  const skipped = broken ? `${broken} file${broken === 1 ? "" : "s"} could not be read` : "";
  const doubled = duplicated.value.length;
  const claimed = doubled ? `${doubled} id${doubled === 1 ? "" : "s"} claimed by more than one file` : "";
  const unsquared = incomplete.value ? "the queue was changing as it was read" : "";
  return [counts, skipped, claimed, unsquared, taskDir.value, version2.value && `tq ${version2.value}`, link].filter(Boolean).join(" · ");
});
var dragging = ref(null);
var composing = ref(null);
var openTaskID = ref(null);
var creating = ref(false);
var openTask = ref(null);
var openTaskMissing = ref(false);
var asked = 0;
watch2([tasks, openTaskID], () => {
  const id = openTaskID.value;
  asked++;
  if (id === null) {
    openTask.value = null;
    openTaskMissing.value = false;
    return;
  }
  const found = tasks.value.find((task) => task.id === id);
  if (found) {
    openTask.value = found;
    openTaskMissing.value = false;
    return;
  }
  confirmMissing(id, asked);
}, { immediate: true, flush: "sync" });
async function confirmMissing(id, ticket) {
  try {
    await fetchTask(id);
    if (ticket === asked)
      openTaskMissing.value = false;
  } catch (error) {
    if (ticket === asked) {
      openTaskMissing.value = error instanceof ApiError && error.status === NOT_FOUND;
    }
  }
}
var busy = computed2(() => dragging.value !== null);
var TOAST_MS = 6000;
var nextToastID = 0;
var toasts = ref([]);
function toast(message, kind = "error") {
  const id = nextToastID++;
  toasts.value = [...toasts.value, { id, kind, message }];
  setTimeout(() => {
    toasts.value = toasts.value.filter((candidate) => candidate.id !== id);
  }, TOAST_MS);
}
var lastPayload = "";
var issued = 0;
async function refresh() {
  const ticket = ++issued;
  const fetched = await fetchTasks();
  if (ticket !== issued)
    return;
  const payload = JSON.stringify(fetched);
  loaded.value = true;
  stale.value = false;
  if (payload === lastPayload)
    return;
  lastPayload = payload;
  tasks.value = fetched;
}
async function refreshQuietly() {
  try {
    await refresh();
  } catch (error) {
    stale.value = true;
    console.error("refresh failed", error);
  }
}
async function moveTask(id, status) {
  const task = tasks.value.find((candidate) => candidate.id === id);
  if (!task || task.status === status)
    return;
  try {
    await patchTask(id, { status });
    await refresh();
  } catch (error) {
    toast(`Could not move ${id}: ${describe(error)}`);
    await refreshQuietly();
  }
}
async function quickAdd(title, status) {
  await createTask({ title, status });
  await refresh();
}
var statusIssued = 0;
async function loadServerStatus() {
  const ticket = ++statusIssued;
  try {
    const status = await fetchStatus();
    if (ticket !== statusIssued)
      return;
    taskDir.value = status.task_dir;
    version2.value = status.version;
    reportMissing(status.unreadable ?? [], status.duplicated ?? []);
    reportIncomplete(status.incomplete ?? false);
  } catch (error) {
    console.error("status failed", error);
  }
}
var complainedAbout = new Set;
var NAMED_IN_TOASTS = 3;
function reportMissing(files, doubled) {
  unreadable.value = files;
  duplicated.value = doubled;
  const seen = new Set([
    ...files.map((file) => `${file.file}: ${file.reason}`),
    ...doubled.map((id) => id.reason)
  ]);
  const fresh = [...seen].filter((complaint) => !complainedAbout.has(complaint));
  complainedAbout = seen;
  for (const complaint of fresh.slice(0, NAMED_IN_TOASTS)) {
    toast(`Not on the board — ${complaint}`);
  }
  const rest = fresh.length - NAMED_IN_TOASTS;
  if (rest > 0)
    toast(`…and ${rest} more problem${rest === 1 ? "" : "s"} the board could not show`);
}
var complainedAboutIncomplete = false;
function reportIncomplete(unsquared) {
  incomplete.value = unsquared;
  if (unsquared && !complainedAboutIncomplete) {
    toast("The queue was changing as it was read — this board may be a task short");
  }
  complainedAboutIncomplete = unsquared;
}
var lastConfig = "";
var lastConfigError = "";
async function loadProjectConfig() {
  let config;
  try {
    config = await fetchConfig();
  } catch (error) {
    const message = describe(error);
    if (message !== lastConfigError) {
      lastConfigError = message;
      toast(`Could not read the project configuration: ${message}`);
    }
    return false;
  }
  lastConfigError = "";
  const payload = JSON.stringify(config);
  if (payload === lastConfig)
    return false;
  lastConfig = payload;
  labels.value = config.labels ?? {};
  if (config.priorities?.length)
    priorities.value = config.priorities;
  if (config.columns?.length)
    columns.value = config.columns;
  return true;
}
async function applySignals(signals) {
  const changed = signals.config ? await loadProjectConfig() : false;
  if (!signals.tasks && !changed)
    return;
  await refreshQuietly();
  await loadServerStatus();
}
async function start() {
  await Promise.all([loadServerStatus(), loadProjectConfig()]);
  try {
    await refresh();
  } catch (error) {
    toast(`Could not load tasks: ${describe(error)}`);
  }
  listen();
  setInterval(() => {
    if (busy.value)
      return;
    if (streaming.value === true && !stale.value)
      return;
    applySignals({ tasks: true, config: true });
  }, POLL_INTERVAL_MS);
}
var queued = { tasks: false, config: false };
function listen() {
  connectEvents({
    onTasks() {
      if (busy.value) {
        queued.tasks = true;
        return;
      }
      applySignals({ tasks: true, config: false });
    },
    onConfig() {
      if (busy.value) {
        queued.config = true;
        return;
      }
      applySignals({ tasks: false, config: true });
    },
    onScanFailed(message) {
      toast(`The server cannot read the queue: ${message}`);
    },
    onConnected(connected) {
      streaming.value = connected;
    }
  });
  watch2(busy, (isBusy) => {
    if (isBusy)
      return;
    const held = { ...queued };
    queued.tasks = false;
    queued.config = false;
    if (held.tasks || held.config)
      applySignals(held);
  });
}

// frontend/notes.ts
var NOTES_HEADING = "## Notes";
var NOTES_RULE = "---";
var FENCE_PATTERN = /^(```|~~~)/;
var HEADING_PATTERN = /^#{1,6}\s/;
var INDENT_PATTERN = /^[ \t]/;
var LIST_MARKER_PATTERN = /^([-*+]|\d{1,9}[.)])\s/;
var BULLET_PATTERN = /^[-*]\s+/;
var INDENT_RUN_PATTERN = /^[ \t]*/;
var NOTE_PATTERN = /^(\S+)\s+—\s+([\s\S]*)$/;
var CONTINUATION_INDENT = "  ";
function trimBlankLines(text) {
  return text.replace(/^\n+|\n+$/g, "");
}
function splitBody(body) {
  const lines = body.split(`
`);
  const start2 = notesStart(lines);
  if (start2 === -1) {
    return { content: trimBlankLines(body), notes: [] };
  }
  let end = start2;
  while (end > 0 && lines[end - 1].trim() === "")
    end--;
  if (end > 0 && lines[end - 1].trim() === NOTES_RULE && (end === 1 || lines[end - 2].trim() === "")) {
    end--;
  }
  return {
    content: trimBlankLines(lines.slice(0, end).join(`
`)),
    notes: parseNotes(lines.slice(start2 + 1))
  };
}
function notesStart(lines) {
  const [start2, balanced] = scanNotesStart(lines, true);
  if (balanced)
    return start2;
  return scanNotesStart(lines, false)[0];
}
function scanNotesStart(lines, honourFences) {
  let start2 = -1;
  let fenced = false;
  let inItem = false;
  for (let i = 0;i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (honourFences && FENCE_PATTERN.test(trimmed)) {
      fenced = !fenced;
      continue;
    }
    const isIndented = INDENT_PATTERN.test(lines[i]);
    if (!isIndented && trimmed !== "")
      inItem = LIST_MARKER_PATTERN.test(trimmed);
    if (fenced || isIndented && inItem || !HEADING_PATTERN.test(trimmed))
      continue;
    start2 = trimmed === NOTES_HEADING ? i : -1;
  }
  return [start2, !fenced];
}
function parseNotes(lines) {
  const notes = [];
  let blanks = [];
  for (const line of lines) {
    const text = line.replace(/\s+$/, "");
    if (text === "") {
      blanks.push("");
      continue;
    }
    const indented = /^\s/.test(text);
    if (notes.length === 0 || !indented && BULLET_PATTERN.test(text)) {
      notes.push(parseNote(text.trim().replace(BULLET_PATTERN, "")));
      blanks = [];
      continue;
    }
    const last = notes[notes.length - 1];
    last.text = [last.text, ...blanks, text.replace(/^ {1,2}/, "")].join(`
`);
    blanks = [];
  }
  return notes;
}
function parseNote(text) {
  const match = NOTE_PATTERN.exec(text);
  if (match && !Number.isNaN(new Date(match[1]).getTime())) {
    return { timestamp: match[1], text: match[2] };
  }
  return { timestamp: "", text };
}
function joinBody(body) {
  const content = trimBlankLines(body.content);
  const notes = body.notes.filter((note) => note.text.trim() !== "");
  if (notes.length === 0)
    return content;
  const section = [NOTES_HEADING, "", ...notes.map(formatNote)].join(`
`);
  return content === "" ? section : [content, "", NOTES_RULE, "", section].join(`
`);
}
function noteLines(text) {
  const lines = [];
  let blank = false;
  for (const raw of text.replace(/\r\n/g, `
`).split(`
`)) {
    const line = raw.replace(/\r$/, "").replace(/[ \t]+$/, "");
    if (line === "") {
      blank = lines.length > 0;
      continue;
    }
    if (blank) {
      lines.push("");
      blank = false;
    }
    lines.push(line);
  }
  if (lines.length === 0)
    return [];
  const indent = commonIndent(lines);
  const stripped = lines.map((line) => line.slice(indent.length));
  stripped[0] = stripped[0].replace(INDENT_RUN_PATTERN, "");
  return stripped;
}
function commonIndent(lines) {
  let indent = null;
  for (const line of lines) {
    if (line === "")
      continue;
    const lead = INDENT_RUN_PATTERN.exec(line)?.[0] ?? "";
    if (indent === null) {
      indent = lead;
      continue;
    }
    let shared = 0;
    while (shared < indent.length && indent[shared] === lead[shared])
      shared++;
    indent = indent.slice(0, shared);
    if (indent === "")
      break;
  }
  return indent ?? "";
}
function formatNote(note) {
  const [first = "", ...rest] = noteLines(note.text);
  const head = note.timestamp === "" ? `- ${first}` : `- ${note.timestamp} — ${first}`;
  return [head, ...rest.map((line) => line === "" ? "" : CONTINUATION_INDENT + line)].join(`
`);
}

// frontend/components/LabelChip.vue?type=script
var _hoisted_1 = ["title"];
var LabelChip_default = /* @__PURE__ */ defineComponent({
  __name: "LabelChip",
  props: {
    name: { type: String, required: true }
  },
  setup(__props) {
    const props = __props;
    const halves = computed2(() => labelHalves(props.name, labels.value));
    const chip = computed2(() => labelChip(props.name, labels.value));
    const scoped = computed2(() => halves.value.scope !== "");
    const pillStyle = computed2(() => {
      const drawn = chip.value;
      if (drawn === null)
        return;
      return scoped.value ? { borderColor: drawn.background } : { background: drawn.background, color: drawn.text };
    });
    const scopeStyle = computed2(() => chip.value === null ? undefined : { background: chip.value.background, color: chip.value.text });
    const valueStyle = computed2(() => chip.value === null ? undefined : { color: chip.value.background });
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("span", {
        class: normalizeClass(["label", { tinted: chip.value !== null, scoped: scoped.value }]),
        style: normalizeStyle(pillStyle.value),
        title: __props.name
      }, [
        scoped.value ? (openBlock(), createElementBlock(Fragment, { key: 0 }, [
          createBaseVNode("span", {
            class: "label-scope",
            style: normalizeStyle(scopeStyle.value)
          }, toDisplayString(halves.value.scope), 5),
          createBaseVNode("span", {
            class: "label-value",
            style: normalizeStyle(valueStyle.value)
          }, toDisplayString(halves.value.value), 5)
        ], 64)) : (openBlock(), createElementBlock(Fragment, { key: 1 }, [
          createTextVNode(toDisplayString(halves.value.value), 1)
        ], 64))
      ], 14, _hoisted_1);
    };
  }
});

// frontend/components/LabelChip.vue
var LabelChip_default2 = LabelChip_default;

// frontend/components/NoteBadge.vue?type=script
var _hoisted_12 = ["title"];
var NoteBadge_default = /* @__PURE__ */ defineComponent({
  __name: "NoteBadge",
  props: {
    count: { type: Number, required: true }
  },
  setup(__props) {
    const props = __props;
    const title = computed2(() => props.count === 1 ? "1 note" : `${props.count} notes`);
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("span", {
        class: "note-badge",
        title: title.value
      }, [
        _cache[0] || (_cache[0] = createBaseVNode("svg", {
          viewBox: "0 0 16 16",
          width: "12",
          height: "12",
          "aria-hidden": "true"
        }, [
          createBaseVNode("path", {
            fill: "currentColor",
            d: "M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v6a1.5 1.5 0 0 1-1.5 1.5H6.6L3.7 13.7A.5.5 0 0 1 3 13.3V11h-.5A1.5 1.5 0 0 1 1 9.5v-6z"
          })
        ], -1)),
        createTextVNode(toDisplayString(__props.count), 1)
      ], 8, _hoisted_12);
    };
  }
});

// frontend/components/NoteBadge.vue
var NoteBadge_default2 = NoteBadge_default;

// frontend/components/PriorityBadge.vue?type=script
var _hoisted_13 = ["title"];
var PriorityBadge_default = /* @__PURE__ */ defineComponent({
  __name: "PriorityBadge",
  props: {
    priority: { type: String, required: false }
  },
  setup(__props) {
    const props = __props;
    const name = computed2(() => props.priority || defaultPriority(priorities.value));
    const display = computed2(() => priorityDisplay(name.value, priorities.value));
    const chip = computed2(() => priorityChip(name.value, priorities.value));
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("span", {
        class: normalizeClass(["badge", { tinted: chip.value !== null }]),
        style: normalizeStyle(chip.value ? { background: chip.value.background, color: chip.value.text } : undefined),
        title: name.value
      }, toDisplayString(display.value), 15, _hoisted_13);
    };
  }
});

// frontend/components/PriorityBadge.vue
var PriorityBadge_default2 = PriorityBadge_default;

// frontend/components/Card.vue?type=script
var _hoisted_14 = ["data-id"];
var _hoisted_2 = { class: "card-top" };
var _hoisted_3 = { class: "task-id" };
var _hoisted_4 = { class: "card-title" };
var _hoisted_5 = {
  key: 0,
  class: "card-meta"
};
var _hoisted_6 = {
  key: 0,
  class: "assignee"
};
var _hoisted_7 = {
  key: 1,
  class: "blocked-note"
};
var Card_default = /* @__PURE__ */ defineComponent({
  __name: "Card",
  props: {
    task: { type: Object, required: true }
  },
  setup(__props) {
    const props = __props;
    const pending = computed2(() => pendingDependencies(props.task, index.value, columns.value));
    const noteCount = computed2(() => splitBody(props.task.body ?? "").notes.length);
    const hasMeta = computed2(() => !!props.task.assignee || (props.task.labels ?? []).length > 0 || noteCount.value > 0);
    function onDragStart(event) {
      dragging.value = props.task.id;
      event.dataTransfer?.setData("text/plain", props.task.id);
      if (event.dataTransfer)
        event.dataTransfer.effectAllowed = "move";
    }
    function onKeydown(event) {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openTaskID.value = props.task.id;
      }
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("article", {
        class: normalizeClass(["card", { blocked: pending.value.length > 0, dragging: unref(dragging) === __props.task.id }]),
        "data-id": __props.task.id,
        draggable: "true",
        tabindex: "0",
        onDragstart: onDragStart,
        onDragend: _cache[0] || (_cache[0] = ($event) => dragging.value = null),
        onClick: _cache[1] || (_cache[1] = ($event) => openTaskID.value = __props.task.id),
        onKeydown
      }, [
        createBaseVNode("div", _hoisted_2, [
          createBaseVNode("span", _hoisted_3, toDisplayString(__props.task.id), 1),
          createVNode(PriorityBadge_default2, {
            priority: __props.task.priority
          }, null, 8, ["priority"])
        ]),
        createBaseVNode("p", _hoisted_4, toDisplayString(__props.task.title), 1),
        hasMeta.value ? (openBlock(), createElementBlock("div", _hoisted_5, [
          __props.task.assignee ? (openBlock(), createElementBlock("span", _hoisted_6, toDisplayString(__props.task.assignee), 1)) : createCommentVNode("v-if", true),
          (openBlock(true), createElementBlock(Fragment, null, renderList(__props.task.labels ?? [], (label) => {
            return openBlock(), createBlock(LabelChip_default2, {
              key: label,
              name: label
            }, null, 8, ["name"]);
          }), 128)),
          noteCount.value > 0 ? (openBlock(), createBlock(NoteBadge_default2, {
            key: 1,
            count: noteCount.value
          }, null, 8, ["count"])) : createCommentVNode("v-if", true)
        ])) : createCommentVNode("v-if", true),
        pending.value.length > 0 ? (openBlock(), createElementBlock("p", _hoisted_7, "Blocked by " + toDisplayString(pending.value.join(", ")), 1)) : createCommentVNode("v-if", true)
      ], 42, _hoisted_14);
    };
  }
});

// frontend/components/Card.vue
var Card_default2 = Card_default;

// frontend/components/Composer.vue?type=script
var _hoisted_15 = { class: "composer" };
var _hoisted_22 = ["onKeydown"];
var Composer_default = /* @__PURE__ */ defineComponent({
  __name: "Composer",
  props: {
    status: { type: String, required: true }
  },
  setup(__props) {
    const props = __props;
    const input = ref(null);
    const draft = ref("");
    let settled = false;
    onMounted(() => {
      input.value?.focus();
    });
    function close() {
      settled = true;
      draft.value = "";
      if (composing.value === props.status)
        composing.value = null;
    }
    async function file(keepOpen) {
      if (settled)
        return;
      const title = draft.value.trim();
      if (title === "") {
        close();
        return;
      }
      settled = true;
      draft.value = "";
      try {
        await quickAdd(title, props.status);
        if (keepOpen) {
          settled = false;
          return;
        }
        close();
      } catch (error) {
        toast(`Could not create the task: ${describe(error)}`);
        settled = false;
        if (composing.value === null)
          composing.value = props.status;
        if (composing.value === props.status) {
          draft.value = title;
          input.value?.focus();
        }
      }
    }
    function onEnter(event, commit) {
      if (event.shiftKey)
        return;
      event.preventDefault();
      commit();
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", _hoisted_15, [
        withDirectives(createBaseVNode("textarea", {
          ref_key: "input",
          ref: input,
          "onUpdate:modelValue": _cache[0] || (_cache[0] = ($event) => draft.value = $event),
          class: "composer-input",
          rows: "2",
          placeholder: "Title",
          onKeydown: [
            _cache[1] || (_cache[1] = withKeys(($event) => onEnter($event, () => file(true)), ["enter"])),
            withKeys(withModifiers(close, ["prevent"]), ["esc"])
          ],
          onBlur: _cache[2] || (_cache[2] = ($event) => file(false))
        }, null, 40, _hoisted_22), [
          [vModelText, draft.value]
        ])
      ]);
    };
  }
});

// frontend/components/Composer.vue
var Composer_default2 = Composer_default;

// frontend/components/Column.vue?type=script
var _hoisted_16 = ["data-status"];
var _hoisted_23 = { class: "column-header" };
var _hoisted_32 = { class: "count" };
var _hoisted_42 = { class: "column-tasks" };
var Column_default = /* @__PURE__ */ defineComponent({
  __name: "Column",
  props: {
    status: { type: String, required: true }
  },
  setup(__props) {
    const props = __props;
    const cards = computed2(() => visible.value.filter((task) => task.status === props.status));
    const heading = computed2(() => columnDisplay(props.status, columns.value));
    const over = ref(false);
    function onDragLeave(event) {
      const to = event.relatedTarget;
      const column = event.currentTarget;
      if (to instanceof Node && column instanceof Node && column.contains(to))
        return;
      over.value = false;
    }
    function onDrop(event) {
      over.value = false;
      const id = event.dataTransfer?.getData("text/plain") || dragging.value;
      if (id)
        moveTask(id, props.status);
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("section", {
        class: normalizeClass(["column", { "drop-target": over.value }]),
        "data-status": __props.status,
        onDragover: _cache[1] || (_cache[1] = withModifiers(($event) => over.value = true, ["prevent"])),
        onDragleave: onDragLeave,
        onDrop: withModifiers(onDrop, ["prevent"])
      }, [
        createBaseVNode("header", _hoisted_23, [
          createBaseVNode("h2", null, toDisplayString(heading.value), 1),
          createBaseVNode("span", _hoisted_32, toDisplayString(cards.value.length), 1)
        ]),
        createBaseVNode("div", _hoisted_42, [
          (openBlock(true), createElementBlock(Fragment, null, renderList(cards.value, (task) => {
            return openBlock(), createBlock(Card_default2, {
              key: task.id,
              task
            }, null, 8, ["task"]);
          }), 128))
        ]),
        unref(composing) === __props.status ? (openBlock(), createBlock(Composer_default2, {
          key: 0,
          status: __props.status
        }, null, 8, ["status"])) : (openBlock(), createElementBlock("button", {
          key: 1,
          type: "button",
          class: "composer-open",
          onClick: _cache[0] || (_cache[0] = ($event) => composing.value = __props.status)
        }, " + Add a card "))
      ], 42, _hoisted_16);
    };
  }
});

// frontend/components/Column.vue
var Column_default2 = Column_default;

// frontend/components/Board.vue?type=script
var Board_default = /* @__PURE__ */ defineComponent({
  __name: "Board",
  setup(__props) {
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("main", {
        id: "board",
        class: "board",
        style: normalizeStyle({ "--column-count": unref(columns).length })
      }, [
        (openBlock(true), createElementBlock(Fragment, null, renderList(unref(columns), (column) => {
          return openBlock(), createBlock(Column_default2, {
            key: column.name,
            status: column.name
          }, null, 8, ["status"]);
        }), 128))
      ], 4);
    };
  }
});

// frontend/components/Board.vue
var Board_default2 = Board_default;

// frontend/format.ts
function splitList(value) {
  return value.split(",").map((item) => item.trim()).filter((item) => item.length > 0);
}
function formatTime(value) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

// frontend/components/CreateDialog.vue?type=script
var _hoisted_17 = { class: "grid" };
var _hoisted_24 = ["value"];
var _hoisted_33 = ["value", "title"];
var CreateDialog_default = /* @__PURE__ */ defineComponent({
  __name: "CreateDialog",
  emits: ["close"],
  setup(__props, { emit: __emit }) {
    const emit2 = __emit;
    const dialog = ref(null);
    const titleField = ref(null);
    const title = ref("");
    const status = ref(defaultColumn(columns.value));
    const priority = ref(defaultPriority(priorities.value));
    const assignee = ref("");
    const labelList = ref("");
    const dependsOn = ref("");
    const body = ref("");
    const priorityChoices = computed2(() => priorityOptions(priorities.value, []));
    onMounted(() => {
      dialog.value?.showModal();
      titleField.value?.focus();
    });
    function dismiss() {
      dialog.value?.close();
    }
    async function submit() {
      const wanted = title.value.trim();
      if (wanted === "")
        return;
      try {
        const task = await createTask({
          title: wanted,
          status: status.value,
          priority: priority.value,
          assignee: assignee.value,
          labels: splitList(labelList.value),
          depends_on: splitList(dependsOn.value),
          body: body.value
        });
        dismiss();
        await refresh();
        toast(`Created ${task.id}`, "info");
      } catch (error) {
        toast(`Could not create the task: ${describe(error)}`);
      }
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("dialog", {
        id: "create-dialog",
        ref_key: "dialog",
        ref: dialog,
        class: "dialog",
        onClose: _cache[7] || (_cache[7] = ($event) => emit2("close"))
      }, [
        createBaseVNode("form", {
          id: "create-form",
          method: "dialog",
          onSubmit: withModifiers(submit, ["prevent"])
        }, [
          createBaseVNode("header", { class: "dialog-header" }, [
            _cache[8] || (_cache[8] = createBaseVNode("span", { class: "task-id" }, "New task", -1)),
            createBaseVNode("button", {
              type: "button",
              class: "ghost close",
              "data-close": "create-dialog",
              "aria-label": "Close",
              onClick: dismiss
            }, " ✕ ")
          ]),
          createBaseVNode("label", null, [
            _cache[9] || (_cache[9] = createTextVNode(" Title ", -1)),
            withDirectives(createBaseVNode("input", {
              id: "create-title",
              ref_key: "titleField",
              ref: titleField,
              "onUpdate:modelValue": _cache[0] || (_cache[0] = ($event) => title.value = $event),
              name: "title",
              type: "text",
              required: ""
            }, null, 512), [
              [vModelText, title.value]
            ])
          ]),
          createBaseVNode("div", _hoisted_17, [
            createBaseVNode("label", null, [
              _cache[10] || (_cache[10] = createTextVNode(" Status ", -1)),
              withDirectives(createBaseVNode("select", {
                id: "create-status",
                "onUpdate:modelValue": _cache[1] || (_cache[1] = ($event) => status.value = $event),
                name: "status"
              }, [
                (openBlock(true), createElementBlock(Fragment, null, renderList(unref(columns), (column) => {
                  return openBlock(), createElementBlock("option", {
                    key: column.name,
                    value: column.name
                  }, toDisplayString(column.display_name), 9, _hoisted_24);
                }), 128))
              ], 512), [
                [vModelSelect, status.value]
              ])
            ]),
            createBaseVNode("label", null, [
              _cache[11] || (_cache[11] = createTextVNode(" Priority ", -1)),
              withDirectives(createBaseVNode("select", {
                id: "create-priority",
                "onUpdate:modelValue": _cache[2] || (_cache[2] = ($event) => priority.value = $event),
                name: "priority"
              }, [
                (openBlock(true), createElementBlock(Fragment, null, renderList(priorityChoices.value, (option) => {
                  return openBlock(), createElementBlock("option", {
                    key: option.name,
                    value: option.name,
                    title: option.name
                  }, toDisplayString(option.display), 9, _hoisted_33);
                }), 128))
              ], 512), [
                [vModelSelect, priority.value]
              ])
            ]),
            createBaseVNode("label", null, [
              _cache[12] || (_cache[12] = createTextVNode(" Assignee ", -1)),
              withDirectives(createBaseVNode("input", {
                id: "create-assignee",
                "onUpdate:modelValue": _cache[3] || (_cache[3] = ($event) => assignee.value = $event),
                name: "assignee",
                type: "text",
                autocomplete: "off"
              }, null, 512), [
                [vModelText, assignee.value]
              ])
            ]),
            createBaseVNode("label", null, [
              _cache[13] || (_cache[13] = createTextVNode(" Labels ", -1)),
              withDirectives(createBaseVNode("input", {
                id: "create-labels",
                "onUpdate:modelValue": _cache[4] || (_cache[4] = ($event) => labelList.value = $event),
                name: "labels",
                type: "text",
                placeholder: "backend, auth",
                autocomplete: "off"
              }, null, 512), [
                [vModelText, labelList.value]
              ])
            ]),
            createBaseVNode("label", null, [
              _cache[14] || (_cache[14] = createTextVNode(" Depends on ", -1)),
              withDirectives(createBaseVNode("input", {
                id: "create-depends-on",
                "onUpdate:modelValue": _cache[5] || (_cache[5] = ($event) => dependsOn.value = $event),
                name: "depends_on",
                type: "text",
                placeholder: "TQ-0002",
                autocomplete: "off"
              }, null, 512), [
                [vModelText, dependsOn.value]
              ])
            ])
          ]),
          createBaseVNode("label", null, [
            _cache[15] || (_cache[15] = createTextVNode(" Body (Markdown) ", -1)),
            withDirectives(createBaseVNode("textarea", {
              id: "create-body",
              "onUpdate:modelValue": _cache[6] || (_cache[6] = ($event) => body.value = $event),
              name: "body",
              rows: "8",
              spellcheck: "false"
            }, null, 512), [
              [vModelText, body.value]
            ])
          ]),
          createBaseVNode("footer", { class: "dialog-footer" }, [
            _cache[16] || (_cache[16] = createBaseVNode("span", { class: "spacer" }, null, -1)),
            createBaseVNode("button", {
              type: "button",
              class: "ghost",
              "data-close": "create-dialog",
              onClick: dismiss
            }, "Cancel"),
            _cache[17] || (_cache[17] = createBaseVNode("button", {
              type: "submit",
              class: "primary"
            }, "Create", -1))
          ])
        ], 32)
      ], 544);
    };
  }
});

// frontend/components/CreateDialog.vue
var CreateDialog_default2 = CreateDialog_default;

// frontend/search.ts
var VALUE_KEYS = ["status", "priority", "label", "assignee"];
var SEARCH_KEYS = [...VALUE_KEYS, "ready"];
var KEY_HINTS = {
  status: "column",
  priority: "level",
  label: "whole label",
  assignee: "substring",
  ready: "unblocked only"
};
var READY = "ready";
var NOT = "-";
var TRUE_WORDS = new Set(["", "true", "yes", "y", "1", "on"]);
var NO_FILTERS = Object.freeze({
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
  text: frozenEmpty(),
  excluded: frozenEmpty()
});
function frozenEmpty() {
  const empty = [];
  Object.freeze(empty);
  return empty;
}
function isSearchKey(name) {
  return SEARCH_KEYS.includes(name);
}
function unquote(text) {
  return text.replaceAll('"', "");
}
function splitAt(raw) {
  let quoted = false;
  for (let at = 0;at < raw.length; at++) {
    const character = raw[at];
    if (character === '"')
      quoted = !quoted;
    else if (character === "=" && !quoted)
      return at;
  }
  return -1;
}
function isQuoted(raw) {
  return raw.startsWith('"');
}
function signOf(raw) {
  if (!raw.startsWith(NOT))
    return { negated: false, body: raw };
  return { negated: true, body: raw.slice(NOT.length) };
}
function readToken(query, start2, end) {
  const raw = query.slice(start2, end);
  const { negated, body } = signOf(raw);
  const at = splitAt(body);
  if (at > 0) {
    const key = unquote(body.slice(0, at)).toLowerCase();
    if (isSearchKey(key))
      return { start: start2, end, raw, negated, key, value: unquote(body.slice(at + 1)) };
  }
  if (!isQuoted(body) && body.toLowerCase() === READY) {
    return { start: start2, end, raw, negated, key: READY, value: "true" };
  }
  return { start: start2, end, raw, negated, key: "", value: unquote(body) };
}
function tokenize(query) {
  const tokens = [];
  let start2 = -1;
  let quoted = false;
  for (let at = 0;at <= query.length; at++) {
    const character = query[at];
    if (character === '"')
      quoted = !quoted;
    const boundary = at === query.length || !quoted && /\s/.test(character ?? "");
    if (boundary) {
      if (start2 >= 0)
        tokens.push(readToken(query, start2, at));
      start2 = -1;
      continue;
    }
    if (start2 < 0)
      start2 = at;
  }
  return tokens;
}
function parseReady(value) {
  return TRUE_WORDS.has(value.trim().toLowerCase());
}
function parseQuery(query) {
  const filters2 = { ...NO_FILTERS, text: [], excluded: [] };
  for (const token of tokenize(query)) {
    const value = token.value.trim();
    if (token.key === "") {
      if (value !== "")
        filters2.text.push({ value, negated: token.negated });
      continue;
    }
    if (token.key === READY) {
      const on = parseReady(token.value);
      filters2.ready = token.negated ? !on : on;
      continue;
    }
    if (token.negated) {
      if (value !== "")
        filters2.excluded.push({ key: token.key, value });
      continue;
    }
    filters2[token.key] = value;
  }
  return filters2;
}
function quoteValue(value) {
  const clean = unquote(value);
  return clean === "" || /\s/.test(clean) ? `"${clean}"` : clean;
}
function sameValue(a, b) {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}
function sameText(a, b) {
  return a.length === b.length && a.every((term, at) => term.negated === b[at].negated && sameValue(term.value, b[at].value));
}
function sameExcluded(a, b) {
  return a.length === b.length && a.every((term, at) => term.key === b[at].key && sameValue(term.value, b[at].value));
}
function equalFilters(a, b) {
  return sameValue(a.status, b.status) && sameValue(a.priority, b.priority) && sameValue(a.assignee, b.assignee) && sameValue(a.label, b.label) && a.ready === b.ready && sameText(a.text, b.text) && sameExcluded(a.excluded, b.excluded);
}
function sameFilters(a, b) {
  return equalFilters(a, b) && CANONICAL_KEYS.every((key) => a[key] === b[key]);
}
var READY_OPTIONS = [{ value: "true" }, { value: "false" }];
var CANONICAL_KEYS = ["status", "priority", "label"];
function canonicalValues(filters2, sources) {
  const corrected = { ...filters2 };
  for (const key of CANONICAL_KEYS) {
    corrected[key] = sources[key].find((option) => sameValue(option.value, filters2[key]))?.value ?? filters2[key];
  }
  return corrected;
}
function keySuggestions(prefix, sign) {
  const wanted = unquote(prefix).toLowerCase();
  return SEARCH_KEYS.filter((key) => key.startsWith(wanted)).map((key) => ({
    kind: "key",
    label: sign + (key === READY ? key : `${key}=`),
    detail: KEY_HINTS[key],
    insert: sign + (key === READY ? key : `${key}=`)
  }));
}
function valueSuggestions(key, prefix, sources, sign) {
  const options = key === READY ? READY_OPTIONS : sources[key];
  const wanted = prefix.trim().toLowerCase();
  const matches = options.filter((option) => option.value.toLowerCase().includes(wanted) || (option.display ?? "").toLowerCase().includes(wanted));
  const ranked = [
    ...matches.filter((option) => option.value.toLowerCase().startsWith(wanted)),
    ...matches.filter((option) => !option.value.toLowerCase().startsWith(wanted))
  ];
  return ranked.map((option) => ({
    kind: "value",
    label: option.value,
    detail: option.display && option.display !== option.value ? option.display : "",
    insert: `${sign}${key}=${quoteValue(option.value)}`
  }));
}
function completeQuery(query, caret, sources) {
  const position = Math.max(0, Math.min(caret, query.length));
  const token = tokenize(query).find((candidate) => position >= candidate.start && position <= candidate.end);
  if (token === undefined) {
    return { start: position, end: position, suggestions: keySuggestions("", "") };
  }
  const sign = token.raw.startsWith(NOT) ? NOT : "";
  const body = token.raw.slice(sign.length);
  const at = splitAt(body);
  const key = at > 0 ? unquote(body.slice(0, at)).toLowerCase() : "";
  if (isSearchKey(key)) {
    return {
      start: token.start,
      end: token.end,
      suggestions: valueSuggestions(key, token.value, sources, sign)
    };
  }
  return { start: token.start, end: token.end, suggestions: keySuggestions(body, sign) };
}
function applyCompletion(query, completion, suggestion) {
  const before = query.slice(0, completion.start);
  const after = query.slice(completion.end);
  const finished = !suggestion.insert.endsWith("=");
  const spaced = finished && !/^\s/.test(after);
  const inserted = spaced ? `${suggestion.insert} ` : suggestion.insert;
  return { query: before + inserted + after, caret: before.length + inserted.length };
}
var QUERY_PARAM = "q";
function queryFromURL(url) {
  try {
    return new URL(url).searchParams.get(QUERY_PARAM) ?? "";
  } catch {
    return "";
  }
}
function urlWithQuery(url, query) {
  let next;
  try {
    next = new URL(url);
  } catch {
    return url;
  }
  if (query.trim() === "")
    next.searchParams.delete(QUERY_PARAM);
  else
    next.searchParams.set(QUERY_PARAM, query);
  return next.pathname + next.search + next.hash;
}

// frontend/components/SearchBar.vue?type=script
var _hoisted_18 = { class: "search" };
var _hoisted_25 = { class: "search-box" };
var _hoisted_34 = ["value", "aria-expanded", "aria-activedescendant"];
var _hoisted_43 = ["id", "aria-selected", "onMousedown", "onMouseenter"];
var _hoisted_52 = { class: "search-option-label" };
var _hoisted_62 = {
  key: 0,
  class: "search-option-detail"
};
var FOCUS_KEY = "/";
var SearchBar_default = /* @__PURE__ */ defineComponent({
  __name: "SearchBar",
  setup(__props) {
    const input = ref(null);
    const menu = ref(null);
    const query = ref(queryFromURL(window.location.href));
    const caret = ref(0);
    const active = ref(0);
    const focused = ref(false);
    const dismissed = ref(false);
    const labelNames = computed2(() => [...new Set([...Object.keys(labels.value), ...labelsInUse(tasks.value)])].filter((name) => name !== "").sort());
    const assignees = computed2(() => [...new Set(tasks.value.map((task) => task.assignee ?? ""))].filter((name) => name !== "").sort());
    const sources = computed2(() => ({
      status: columns.value.map((column) => ({ value: column.name, display: column.display_name })),
      priority: priorities.value.map((priority) => ({
        value: priority.name,
        display: priority.display_name
      })),
      label: labelNames.value.map((name) => ({ value: name, display: labelDisplay(name, labels.value) })),
      assignee: assignees.value.map((name) => ({ value: name }))
    }));
    const parsed = computed2(() => canonicalValues(parseQuery(query.value), sources.value));
    watch2(parsed, (next) => {
      if (sameFilters(next, filters))
        return;
      Object.assign(filters, next);
    }, { immediate: true });
    watch2(query, (line) => {
      window.history.replaceState(null, "", urlWithQuery(window.location.href, line));
    });
    const completion = computed2(() => completeQuery(query.value, caret.value, sources.value));
    const showing = computed2(() => focused.value && !dismissed.value && completion.value.suggestions.length > 0);
    watch2(completion, () => {
      active.value = 0;
    });
    watch2(active, async (at) => {
      await nextTick();
      const row = menu.value?.children[at];
      if (row instanceof HTMLElement)
        row.scrollIntoView({ block: "nearest" });
    });
    function syncCaret() {
      const field = input.value;
      if (field)
        caret.value = field.selectionStart ?? field.value.length;
    }
    function onInput(event) {
      const field = event.target;
      query.value = field.value;
      caret.value = field.selectionStart ?? field.value.length;
      dismissed.value = false;
    }
    function focusInput() {
      const field = input.value;
      if (field === null)
        return;
      field.focus();
      const end = field.value.length;
      field.setSelectionRange(end, end);
      caret.value = end;
    }
    function clear() {
      query.value = "";
      dismissed.value = false;
      nextTick(focusInput);
    }
    async function accept(at) {
      const suggestion = completion.value.suggestions[at];
      if (suggestion === undefined)
        return;
      const next = applyCompletion(query.value, completion.value, suggestion);
      query.value = next.query;
      caret.value = next.caret;
      dismissed.value = false;
      await nextTick();
      const field = input.value;
      if (field === null)
        return;
      field.focus();
      field.setSelectionRange(next.caret, next.caret);
    }
    function onKeydown(event) {
      if (event.key === "Escape") {
        if (!showing.value)
          return;
        dismissed.value = true;
        event.preventDefault();
        event.stopPropagation();
        return;
      }
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        dismissed.value = false;
        const count = completion.value.suggestions.length;
        if (count === 0)
          return;
        event.preventDefault();
        const step = event.key === "ArrowDown" ? 1 : -1;
        active.value = (active.value + step + count) % count;
        return;
      }
      if (event.key === "Enter" && showing.value) {
        event.preventDefault();
        accept(active.value);
      }
    }
    function isTyping(target) {
      if (!(target instanceof HTMLElement))
        return false;
      const tag = target.tagName;
      return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
    }
    function onShortcut(event) {
      if (event.key !== FOCUS_KEY || event.isComposing)
        return;
      if (event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented)
        return;
      if (isTyping(event.target))
        return;
      if (document.querySelector("dialog[open]") !== null)
        return;
      event.preventDefault();
      focusInput();
    }
    onMounted(() => document.addEventListener("keydown", onShortcut));
    onBeforeUnmount(() => document.removeEventListener("keydown", onShortcut));
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", _hoisted_18, [
        createBaseVNode("div", _hoisted_25, [
          createBaseVNode("input", {
            id: "search-query",
            ref_key: "input",
            ref: input,
            value: query.value,
            type: "text",
            role: "combobox",
            class: "search-input",
            placeholder: "text, -not, or priority=urgent  (/)",
            autocomplete: "off",
            spellcheck: "false",
            "aria-label": "Search",
            "aria-autocomplete": "list",
            "aria-controls": "search-suggestions",
            "aria-expanded": showing.value,
            "aria-activedescendant": showing.value ? `search-option-${active.value}` : undefined,
            onInput,
            onKeyup: syncCaret,
            onClick: syncCaret,
            onKeydown,
            onFocus: _cache[0] || (_cache[0] = ($event) => focused.value = true),
            onBlur: _cache[1] || (_cache[1] = ($event) => focused.value = false)
          }, null, 40, _hoisted_34),
          query.value !== "" ? (openBlock(), createElementBlock("button", {
            key: 0,
            id: "search-clear",
            type: "button",
            class: "search-clear",
            title: "Clear the search",
            "aria-label": "Clear the search",
            onClick: clear
          }, "×")) : createCommentVNode("v-if", true)
        ]),
        showing.value ? (openBlock(), createElementBlock("ul", {
          key: 0,
          id: "search-suggestions",
          ref_key: "menu",
          ref: menu,
          class: "search-menu",
          role: "listbox"
        }, [
          (openBlock(true), createElementBlock(Fragment, null, renderList(completion.value.suggestions, (suggestion, at) => {
            return openBlock(), createElementBlock("li", {
              id: `search-option-${at}`,
              key: suggestion.insert,
              class: normalizeClass(["search-option", { active: at === active.value }]),
              role: "option",
              "aria-selected": at === active.value,
              onMousedown: withModifiers(($event) => accept(at), ["prevent"]),
              onMouseenter: ($event) => active.value = at
            }, [
              createBaseVNode("span", _hoisted_52, toDisplayString(suggestion.label), 1),
              suggestion.detail ? (openBlock(), createElementBlock("span", _hoisted_62, toDisplayString(suggestion.detail), 1)) : createCommentVNode("v-if", true)
            ], 42, _hoisted_43);
          }), 128))
        ], 512)) : createCommentVNode("v-if", true)
      ]);
    };
  }
});

// frontend/components/SearchBar.vue
var SearchBar_default2 = SearchBar_default;

// frontend/edit.ts
function commitField(opened, edited, current) {
  if (edited === opened)
    return "unchanged";
  return current === opened ? "write" : "conflict";
}
function commitContent(opened, content, current) {
  if (content === opened)
    return { outcome: "unchanged" };
  if (current.content !== opened)
    return { outcome: "conflict" };
  return { outcome: "write", body: joinBody({ content, notes: current.notes }) };
}
function commitNote(opened, text, current) {
  if (text === opened.text)
    return { outcome: "unchanged" };
  const at = current.notes.findIndex((note) => note.timestamp === opened.timestamp && note.text === opened.text);
  if (at === -1)
    return { outcome: "conflict" };
  const notes = current.notes.map((note, position) => position === at ? { timestamp: note.timestamp, text } : note);
  return { outcome: "write", body: joinBody({ content: current.content, notes }) };
}

// frontend/components/InlineText.vue?type=script
var _hoisted_19 = ["id", "aria-label", "onKeydown"];
var _hoisted_26 = ["id", "aria-label", "onKeydown"];
var _hoisted_35 = {
  key: 2,
  class: "inline-actions"
};
var InlineText_default = /* @__PURE__ */ defineComponent({
  __name: "InlineText",
  props: {
    value: { type: String, required: true },
    id: { type: String, required: true },
    label: { type: String, required: true },
    heading: { type: Boolean, required: false, default: false },
    multiline: { type: Boolean, required: false, default: false },
    placeholder: { type: String, required: false, default: "" },
    commit: { type: Function, required: true }
  },
  emits: ["open", "close"],
  setup(__props, { emit: __emit }) {
    const props = __props;
    const emit2 = __emit;
    const editing = ref(false);
    const draft = ref("");
    const editor = useTemplateRef("editor");
    let writing = false;
    function begin() {
      if (editing.value)
        return;
      draft.value = props.value;
      editing.value = true;
      emit2("open");
      nextTick(() => {
        const area = editor.value;
        if (!area)
          return;
        area.focus();
        area.setSelectionRange(area.value.length, area.value.length);
      });
    }
    function close() {
      editing.value = false;
      emit2("close");
    }
    async function finish(keep) {
      if (!editing.value || writing)
        return;
      if (!keep || draft.value === props.value) {
        close();
        return;
      }
      writing = true;
      try {
        if (await props.commit(draft.value))
          close();
      } finally {
        writing = false;
      }
    }
    function cancel() {
      if (!editing.value)
        return;
      close();
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", {
        class: normalizeClass(["inline-text", { multiline: __props.multiline, editing: editing.value }])
      }, [
        editing.value ? (openBlock(), createElementBlock(Fragment, { key: 0 }, [
          __props.multiline ? withDirectives((openBlock(), createElementBlock("textarea", {
            key: 0,
            id: `${__props.id}-edit`,
            ref_key: "editor",
            ref: editor,
            "onUpdate:modelValue": _cache[0] || (_cache[0] = ($event) => draft.value = $event),
            class: "inline-editor",
            rows: "12",
            spellcheck: "false",
            "aria-label": __props.label,
            onKeydown: [
              withKeys(withModifiers(cancel, ["prevent", "stop"]), ["esc"]),
              _cache[1] || (_cache[1] = withKeys(withModifiers(($event) => finish(true), ["ctrl", "prevent"]), ["enter"])),
              _cache[2] || (_cache[2] = withKeys(withModifiers(($event) => finish(true), ["meta", "prevent"]), ["enter"]))
            ],
            onBlur: _cache[3] || (_cache[3] = ($event) => finish(true))
          }, null, 40, _hoisted_19)), [
            [vModelText, draft.value]
          ]) : withDirectives((openBlock(), createElementBlock("input", {
            key: 1,
            id: `${__props.id}-edit`,
            ref_key: "editor",
            ref: editor,
            "onUpdate:modelValue": _cache[4] || (_cache[4] = ($event) => draft.value = $event),
            class: "inline-editor",
            type: "text",
            autocomplete: "off",
            "aria-label": __props.label,
            onKeydown: [
              _cache[5] || (_cache[5] = withKeys(withModifiers(($event) => finish(true), ["prevent"]), ["enter"])),
              withKeys(withModifiers(cancel, ["prevent", "stop"]), ["esc"])
            ],
            onBlur: _cache[6] || (_cache[6] = ($event) => finish(true))
          }, null, 40, _hoisted_26)), [
            [vModelText, draft.value]
          ]),
          __props.multiline ? (openBlock(), createElementBlock("div", _hoisted_35, [
            createBaseVNode("button", {
              type: "button",
              class: "primary",
              onMousedown: _cache[7] || (_cache[7] = withModifiers(() => {}, ["prevent"])),
              onClick: _cache[8] || (_cache[8] = ($event) => finish(true))
            }, "Save", 32),
            createCommentVNode(` Both handlers, and both load-bearing: mousedown is what stops the
             textarea blurring (which would write the draft this button exists
             to drop), and click is the only one a keyboard fires. `),
            createBaseVNode("button", {
              type: "button",
              class: "ghost",
              onMousedown: withModifiers(cancel, ["prevent"]),
              onClick: cancel
            }, " Cancel ", 32)
          ])) : createCommentVNode("v-if", true)
        ], 64)) : (openBlock(), createBlock(resolveDynamicComponent(__props.heading ? "h2" : "div"), {
          key: 1,
          id: __props.id,
          class: normalizeClass(["inline-value", { empty: __props.value === "" }]),
          role: "button",
          tabindex: "0",
          title: `Click to edit the ${__props.label.toLowerCase()}`,
          onClick: begin,
          onKeydown: withKeys(withModifiers(begin, ["prevent"]), ["enter"])
        }, {
          default: withCtx(() => [
            renderSlot(_ctx.$slots, "default", {}, () => [
              createTextVNode(toDisplayString(__props.value === "" ? __props.placeholder : __props.value), 1)
            ])
          ]),
          _: 3
        }, 40, ["id", "class", "title", "onKeydown"]))
      ], 2);
    };
  }
});

// frontend/components/InlineText.vue
var InlineText_default2 = InlineText_default;

// frontend/markdown.ts
var SAFE_SCHEME = /^(https?:|mailto:)/i;
var HAS_SCHEME = /^[a-z][a-z0-9+.-]*:/i;
var INVISIBLE = /[\u0000-\u001f\u007f]/g;
var FENCE = /^(`{3,}|~{3,})\s*(\S*)/;
var HEADING = /^(#{1,6})\s+(.*)$/;
var RULE = /^(-{3,}|\*{3,}|_{3,})$/;
var QUOTE = /^>\s?(.*)$/;
var BULLET = /^([-*+])\s+(.*)$/;
var NUMBER = /^(\d{1,9})[.)]\s+(.*)$/;
var CHECKBOX = /^\[([ xX])\]\s+(.*)$/;
var INDENT = /^[ \t]*/;
var TAB_WIDTH = 4;
function renderMarkdown(source) {
  return blocks(source.replace(/\r\n?/g, `
`).split(`
`));
}
function indentOf(line) {
  let width = 0;
  for (const character of INDENT.exec(line)?.[0] ?? "") {
    width += character === "\t" ? TAB_WIDTH - width % TAB_WIDTH : 1;
  }
  return width;
}
function prefixLength(line, column) {
  let width = 0;
  let at = 0;
  while (at < line.length && width < column) {
    const character = line[at];
    if (character !== " " && character !== "\t")
      break;
    width += character === "\t" ? TAB_WIDTH - width % TAB_WIDTH : 1;
    at++;
  }
  return at;
}
function blocks(lines) {
  const out = [];
  let at = 0;
  while (at < lines.length) {
    const trimmed = lines[at].trim();
    if (trimmed === "") {
      at++;
      continue;
    }
    const fence = FENCE.exec(trimmed);
    if (fence) {
      const closed = fenceEnd(lines, at + 1, fence[1][0]);
      out.push(codeBlock(lines.slice(at + 1, closed), fence[2]));
      at = closed < lines.length ? closed + 1 : closed;
      continue;
    }
    const heading = HEADING.exec(trimmed);
    if (heading) {
      const level = heading[1].length;
      out.push(`<h${level}>${inline(heading[2].trim())}</h${level}>`);
      at++;
      continue;
    }
    if (RULE.test(trimmed)) {
      out.push("<hr>");
      at++;
      continue;
    }
    if (QUOTE.test(trimmed)) {
      const end2 = runOf(lines, at, (line) => QUOTE.test(line.trim()));
      const inner = lines.slice(at, end2).map((line) => QUOTE.exec(line.trim())?.[1] ?? "");
      out.push(`<blockquote>${blocks(inner)}</blockquote>`);
      at = end2;
      continue;
    }
    if (BULLET.test(trimmed) || NUMBER.test(trimmed)) {
      const list = listAt(lines, at, indentOf(lines[at]));
      out.push(list.html);
      at = list.next;
      continue;
    }
    const end = runOf(lines, at, (line) => line.trim() !== "" && !opensABlock(line.trim()));
    const text = lines.slice(at, end).map((line) => line.trim()).join(`
`);
    out.push(`<p>${inline(text)}</p>`);
    at = end;
  }
  return out.join("");
}
function opensABlock(trimmed) {
  return FENCE.test(trimmed) || HEADING.test(trimmed) || RULE.test(trimmed) || QUOTE.test(trimmed) || BULLET.test(trimmed) || NUMBER.test(trimmed);
}
function runOf(lines, at, holds) {
  let end = at + 1;
  while (end < lines.length && holds(lines[end]))
    end++;
  return end;
}
function fenceEnd(lines, from, marker) {
  for (let at = from;at < lines.length; at++) {
    const found = FENCE.exec(lines[at].trim());
    if (found && found[1][0] === marker)
      return at;
  }
  return lines.length;
}
function codeBlock(lines, language) {
  const marked = language === "" ? "" : ` class="language-${escapeHTML(language)}"`;
  return `<pre><code${marked}>${escapeHTML(lines.join(`
`))}</code></pre>`;
}
function listAt(lines, at, column) {
  const first = lines[at].trim();
  const ordered = !BULLET.test(first) && NUMBER.test(first);
  const start2 = ordered ? Number(NUMBER.exec(first)?.[1] ?? "1") : 1;
  const items = [];
  let position = at;
  while (position < lines.length) {
    if (lines[position].trim() === "") {
      const next = runOf(lines, position, (line) => line.trim() === "");
      if (next >= lines.length || indentOf(lines[next]) < column)
        break;
      position = next;
      continue;
    }
    if (indentOf(lines[position]) !== column)
      break;
    const marker = (ordered ? NUMBER : BULLET).exec(lines[position].trim());
    if (!marker)
      break;
    let end = position + 1;
    while (end < lines.length) {
      if (lines[end].trim() !== "" && indentOf(lines[end]) <= column)
        break;
      end++;
    }
    while (end > position + 1 && lines[end - 1].trim() === "")
      end--;
    const text = column + marker[0].length - marker[2].length;
    items.push(item([marker[2], ...lines.slice(position + 1, end)], text));
    position = end;
  }
  const tag = ordered ? "ol" : "ul";
  const from = ordered && start2 !== 1 ? ` start="${start2}"` : "";
  return { html: `<${tag}${from}>${items.join("")}</${tag}>`, next: position };
}
function item(body, column) {
  const [head = "", ...rest] = body;
  const box = CHECKBOX.exec(head);
  const checked = box ? ` <input type="checkbox" disabled${box[1] === " " ? "" : " checked"}>` : "";
  const inner = blocks([
    box ? box[2] : head,
    ...rest.map((line) => line.slice(prefixLength(line, column)))
  ]);
  const single = (inner.match(/<p>/g) ?? []).length === 1;
  const only = single ? inner.replace(/^<p>([\s\S]*?)<\/p>/, "$1") : inner;
  return `<li${box ? ' class="task-item"' : ""}>${checked}${only}</li>`;
}
function escapeHTML(text) {
  return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
var HELD = "\x00";
function inline(text) {
  const code = [];
  const held = escapeHTML(text.replace(new RegExp(HELD, "g"), "")).replace(/(`+)([\s\S]*?)\1/g, (_whole, _ticks, inner) => {
    code.push(`<code>${trimCodeSpan(inner)}</code>`);
    return `${HELD}${code.length - 1}${HELD}`;
  });
  const marked = held.replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (whole, alt, href) => {
    const url = safeHref(href);
    return url === null ? whole : `<img src="${url}" alt="${alt}" loading="lazy">`;
  }).replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (whole, label, href) => {
    const url = safeHref(href);
    return url === null ? whole : `${anchor(url)}${label}</a>`;
  }).replace(/(^|[\s(])(https?:\/\/[^\s<)]+)/g, (_whole, before, url) => `${before}${anchor(url)}${url}</a>`).replace(/(\*\*|__)(?=\S)([\s\S]*?\S)\1/g, "<strong>$2</strong>").replace(/(?<![*\w])\*(?=\S)([^*]*?\S)\*(?![*\w])/g, "<em>$1</em>").replace(/(?<!\w)_(?=\S)([^_]*?\S)_(?!\w)/g, "<em>$1</em>").replace(/~~(?=\S)([\s\S]*?\S)~~/g, "<del>$1</del>").replace(/\n/g, "<br>");
  return marked.replace(new RegExp(`${HELD}(\\d+)${HELD}`, "g"), (_whole, at) => code[Number(at)]);
}
function trimCodeSpan(text) {
  return /^ [\s\S]*[^ ] $/.test(text) ? text.slice(1, -1) : text;
}
function safeHref(href) {
  const cleaned = href.replace(INVISIBLE, "");
  return HAS_SCHEME.test(cleaned) && !SAFE_SCHEME.test(cleaned) ? null : cleaned;
}
function anchor(url) {
  return `<a href="${url}" target="_blank" rel="noreferrer noopener">`;
}

// frontend/components/Markdown.vue?type=script
var _hoisted_110 = ["innerHTML"];
var Markdown_default = /* @__PURE__ */ defineComponent({
  __name: "Markdown",
  props: {
    source: { type: String, required: true }
  },
  setup(__props) {
    const props = __props;
    const html = computed2(() => renderMarkdown(props.source));
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", {
        class: "markdown",
        innerHTML: html.value
      }, null, 8, _hoisted_110);
    };
  }
});

// frontend/components/Markdown.vue
var Markdown_default2 = Markdown_default;

// frontend/components/NotesPanel.vue?type=script
var _hoisted_111 = { class: "notes-section" };
var _hoisted_27 = { class: "note-row" };
var _hoisted_36 = {
  key: 0,
  class: "notes-empty"
};
var _hoisted_44 = {
  key: 1,
  class: "note detached"
};
var _hoisted_53 = { class: "note-head" };
var _hoisted_63 = { class: "note-time" };
var _hoisted_72 = ["onMousedown", "onClick"];
var _hoisted_8 = ["onKeydown", "onBlur"];
var _hoisted_9 = {
  key: 1,
  class: "note-text"
};
var NotesPanel_default = /* @__PURE__ */ defineComponent({
  __name: "NotesPanel",
  props: {
    notes: { type: Array, required: true },
    commit: { type: Function, required: true },
    append: { type: Function, required: true }
  },
  setup(__props) {
    const props = __props;
    const list = useTemplateRef("list");
    const draft = ref("");
    const editing = ref(null);
    const editor = ref("");
    let writing = false;
    function same(one, other) {
      return one !== null && one.timestamp === other.timestamp && one.text === other.text;
    }
    const editingAt = computed2(() => props.notes.findIndex((note) => same(editing.value, note)));
    const detached = computed2(() => editing.value !== null && editingAt.value === -1);
    async function write(note, raw) {
      if (writing)
        return false;
      const text = noteLines(raw).join(`
`);
      if (text === "" || text === note.text)
        return true;
      writing = true;
      try {
        return await props.commit(note, text);
      } finally {
        writing = false;
      }
    }
    async function beginEdit(note) {
      if (same(editing.value, note))
        return;
      if (editing.value !== null && !await write(editing.value, editor.value))
        return;
      editing.value = { ...note };
      editor.value = note.text;
      await nextTick();
      const area = list.value?.querySelector("textarea.note-editor");
      if (area instanceof HTMLTextAreaElement) {
        area.focus();
        area.setSelectionRange(area.value.length, area.value.length);
      }
    }
    async function finish(keep, note) {
      if (!same(editing.value, note))
        return;
      if (!keep) {
        editing.value = null;
        return;
      }
      if (await write(note, editor.value))
        editing.value = null;
    }
    async function send() {
      const text = draft.value;
      if (text.trim() === "" || writing)
        return;
      writing = true;
      try {
        if (await props.append(text))
          draft.value = "";
      } finally {
        writing = false;
      }
    }
    function onEnter(event, act) {
      if (event.shiftKey)
        return;
      event.preventDefault();
      act();
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("section", _hoisted_111, [
        _cache[6] || (_cache[6] = createBaseVNode("h3", { class: "task-section" }, "Notes", -1)),
        createBaseVNode("div", _hoisted_27, [
          withDirectives(createBaseVNode("textarea", {
            id: "task-note",
            "onUpdate:modelValue": _cache[0] || (_cache[0] = ($event) => draft.value = $event),
            class: "note-draft",
            rows: "2",
            placeholder: "Write a note…",
            onKeydown: _cache[1] || (_cache[1] = withKeys(($event) => onEnter($event, send), ["enter"]))
          }, null, 544), [
            [vModelText, draft.value]
          ]),
          createBaseVNode("button", {
            id: "task-note-add",
            type: "button",
            class: "primary",
            onClick: send
          }, "Send")
        ]),
        createBaseVNode("ul", {
          id: "task-notes",
          ref_key: "list",
          ref: list,
          class: "notes"
        }, [
          __props.notes.length === 0 && !detached.value ? (openBlock(), createElementBlock("li", _hoisted_36, "No notes yet.")) : createCommentVNode("v-if", true),
          detached.value && editing.value !== null ? (openBlock(), createElementBlock("li", _hoisted_44, [
            _cache[5] || (_cache[5] = createBaseVNode("p", {
              id: "task-note-detached",
              class: "note-moved"
            }, " The note you were editing is no longer in the file as you opened it. Nothing was written — copy what you need, then press Escape. ", -1)),
            withDirectives(createBaseVNode("textarea", {
              "onUpdate:modelValue": _cache[2] || (_cache[2] = ($event) => editor.value = $event),
              class: "note-editor",
              rows: "2",
              onKeydown: _cache[3] || (_cache[3] = withKeys(withModifiers(($event) => editing.value = null, ["prevent", "stop"]), ["esc"]))
            }, null, 544), [
              [vModelText, editor.value]
            ])
          ])) : createCommentVNode("v-if", true),
          (openBlock(true), createElementBlock(Fragment, null, renderList(__props.notes, (note, position) => {
            return openBlock(), createElementBlock("li", {
              key: position,
              class: "note"
            }, [
              createBaseVNode("div", _hoisted_53, [
                createBaseVNode("time", _hoisted_63, toDisplayString(note.timestamp === "" ? "note" : unref(formatTime)(note.timestamp)), 1),
                createBaseVNode("button", {
                  type: "button",
                  class: "ghost icon",
                  title: "Edit this note",
                  "aria-label": "Edit this note",
                  onMousedown: withModifiers(($event) => beginEdit(note), ["prevent"]),
                  onClick: ($event) => beginEdit(note)
                }, " ✎ ", 40, _hoisted_72)
              ]),
              editingAt.value === position ? withDirectives((openBlock(), createElementBlock("textarea", {
                key: 0,
                "onUpdate:modelValue": _cache[4] || (_cache[4] = ($event) => editor.value = $event),
                class: "note-editor",
                rows: "2",
                onKeydown: [
                  withKeys(($event) => onEnter($event, () => finish(true, note)), ["enter"]),
                  withKeys(withModifiers(($event) => finish(false, note), ["prevent", "stop"]), ["esc"])
                ],
                onBlur: ($event) => finish(true, note)
              }, null, 40, _hoisted_8)), [
                [vModelText, editor.value]
              ]) : (openBlock(), createElementBlock("p", _hoisted_9, toDisplayString(note.text), 1))
            ]);
          }), 128))
        ], 512)
      ]);
    };
  }
});

// frontend/components/NotesPanel.vue
var NotesPanel_default2 = NotesPanel_default;

// frontend/components/TokenField.vue?type=script
var _hoisted_112 = ["id"];
var _hoisted_28 = ["title", "aria-label", "onClick"];
var _hoisted_37 = ["id", "placeholder", "aria-label", "onKeydown"];
var _hoisted_45 = ["id", "title", "aria-label"];
var TokenField_default = /* @__PURE__ */ defineComponent({
  __name: "TokenField",
  props: {
    values: { type: Array, required: true },
    id: { type: String, required: true },
    label: { type: String, required: true },
    placeholder: { type: String, required: false },
    commit: { type: Function, required: true }
  },
  setup(__props) {
    const props = __props;
    const adding = ref(false);
    const draft = ref("");
    const input = useTemplateRef("input");
    let writing = false;
    function begin() {
      adding.value = true;
      draft.value = "";
      nextTick(() => input.value?.focus());
    }
    async function add() {
      if (writing)
        return;
      const value = draft.value.trim();
      if (value === "" || props.values.includes(value)) {
        adding.value = false;
        return;
      }
      writing = true;
      try {
        if (await props.commit([...props.values, value]))
          adding.value = false;
      } finally {
        writing = false;
      }
    }
    async function remove2(value) {
      if (writing)
        return;
      writing = true;
      try {
        await props.commit(props.values.filter((candidate) => candidate !== value));
      } finally {
        writing = false;
      }
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", {
        id: __props.id,
        class: "tokens"
      }, [
        (openBlock(true), createElementBlock(Fragment, null, renderList(__props.values, (value) => {
          return openBlock(), createElementBlock("span", {
            key: value,
            class: "token"
          }, [
            renderSlot(_ctx.$slots, "default", { value }, () => [
              createTextVNode(toDisplayString(value), 1)
            ]),
            createBaseVNode("button", {
              type: "button",
              class: "ghost icon token-remove",
              title: `Remove ${value}`,
              "aria-label": `Remove ${value}`,
              onClick: ($event) => remove2(value)
            }, " ✕ ", 8, _hoisted_28)
          ]);
        }), 128)),
        adding.value ? withDirectives((openBlock(), createElementBlock("input", {
          key: 0,
          id: `${__props.id}-input`,
          ref_key: "input",
          ref: input,
          "onUpdate:modelValue": _cache[0] || (_cache[0] = ($event) => draft.value = $event),
          class: "token-input",
          type: "text",
          autocomplete: "off",
          placeholder: __props.placeholder,
          "aria-label": `Add ${__props.label}`,
          onKeydown: [
            withKeys(withModifiers(add, ["prevent"]), ["enter"]),
            _cache[1] || (_cache[1] = withKeys(withModifiers(($event) => adding.value = false, ["prevent", "stop"]), ["esc"]))
          ],
          onBlur: add
        }, null, 40, _hoisted_37)), [
          [vModelText, draft.value]
        ]) : (openBlock(), createElementBlock("button", {
          key: 1,
          id: `${__props.id}-add`,
          type: "button",
          class: "ghost icon token-add",
          title: `Add ${__props.label}`,
          "aria-label": `Add ${__props.label}`,
          onClick: begin
        }, " ＋ ", 8, _hoisted_45))
      ], 8, _hoisted_112);
    };
  }
});

// frontend/components/TokenField.vue
var TokenField_default2 = TokenField_default;

// frontend/components/TaskDialog.vue?type=script
var _hoisted_113 = { class: "task-sheet" };
var _hoisted_29 = { class: "task-head" };
var _hoisted_38 = { class: "task-head-text" };
var _hoisted_46 = {
  id: "task-dialog-id",
  class: "task-id"
};
var _hoisted_54 = ["value"];
var _hoisted_64 = ["value"];
var _hoisted_73 = ["hidden"];
var _hoisted_82 = ["hidden"];
var _hoisted_92 = { class: "task-columns" };
var _hoisted_10 = { class: "task-main" };
var _hoisted_11 = {
  key: 1,
  class: "task-empty"
};
var _hoisted_122 = { class: "token-id" };
var _hoisted_132 = ["hidden"];
var _hoisted_142 = { class: "task-side" };
var _hoisted_152 = ["value"];
var _hoisted_162 = ["value", "title"];
var _hoisted_172 = {
  id: "task-timestamps",
  class: "timestamps"
};
var EDITORS = ".inline-editor, .note-editor, .token-input";
var TaskDialog_default = /* @__PURE__ */ defineComponent({
  __name: "TaskDialog",
  props: {
    task: { type: Object, required: true }
  },
  emits: ["close"],
  setup(__props, { emit: __emit }) {
    const props = __props;
    const emit2 = __emit;
    const dialog = ref(null);
    const split = computed2(() => splitBody(props.task.body ?? ""));
    const priority = computed2(() => props.task.priority || defaultPriority(priorities.value));
    const priorityChoices = computed2(() => priorityOptions(priorities.value, [priority.value]));
    const pending = computed2(() => pendingDependencies(props.task, index.value, columns.value));
    const timestamps = computed2(() => `created ${formatTime(props.task.created)} · updated ${formatTime(props.task.updated)}`);
    onMounted(() => dialog.value?.showModal());
    function dismiss() {
      dialog.value?.close();
    }
    let pressedOutside = false;
    function onMouseDown(event) {
      pressedOutside = event.target === dialog.value;
    }
    function onClick(event) {
      const outside = pressedOutside && event.target === dialog.value;
      pressedOutside = false;
      if (outside && dialog.value?.querySelector(EDITORS) === null)
        dismiss();
    }
    const TITLE = { what: "title", of: (task) => task.title };
    const ASSIGNEE = { what: "assignee", of: (task) => task.assignee ?? "" };
    const DESCRIPTION = {
      what: "description",
      of: (task) => splitBody(task.body ?? "").content
    };
    const editing = ref(null);
    const opened = ref("");
    function begin(field) {
      editing.value = field;
      opened.value = field.of(props.task);
    }
    const moved = computed2(() => {
      const field = editing.value;
      if (field === null)
        return "";
      return field.of(props.task) === opened.value ? "" : field.what;
    });
    function refuse(what) {
      toast(`Not saved: the ${what} of ${props.task.id} changed on disk while you were editing it. ` + `Nothing was written — copy what you need, then press Escape to see the file's version, ` + `and check the change in your VCS.`);
    }
    async function writeField(field, was, edited, patch) {
      try {
        const commit = commitField(was, edited, field.of(await fetchTask(props.task.id)));
        if (commit === "unchanged")
          return true;
        if (commit === "conflict") {
          refuse(field.what);
          return false;
        }
        await patchTask(props.task.id, patch);
        await refresh();
        return true;
      } catch (error) {
        toast(`Could not update ${props.task.id}: ${describe(error)}`);
        return false;
      }
    }
    async function writeBody(what, decide) {
      try {
        const commit = decide(splitBody((await fetchTask(props.task.id)).body ?? ""));
        if (commit.outcome === "conflict") {
          refuse(what);
          return false;
        }
        if (commit.outcome === "write") {
          await patchTask(props.task.id, { body: commit.body });
          await refresh();
        }
        return true;
      } catch (error) {
        toast(`Could not update ${props.task.id}: ${describe(error)}`);
        return false;
      }
    }
    async function saveTitle(title) {
      if (title.trim() === "") {
        toast(`Not saved: ${props.task.id} needs a title.`);
        return false;
      }
      return writeField(TITLE, opened.value, title, { title });
    }
    const saveAssignee = (assignee) => writeField(ASSIGNEE, opened.value, assignee, { assignee });
    const saveContent = (content) => writeBody(DESCRIPTION.what, (current) => commitContent(opened.value, content, current));
    const saveNote = (note, text) => writeBody("note", (current) => commitNote(note, text, current));
    async function choose(field, event, patch) {
      const select = event.target;
      const chosen = select.value;
      const was = field.of(props.task);
      if (!await writeField(field, was, chosen, patch(chosen)))
        select.value = was;
    }
    const chooseStatus = (event) => choose({ what: "status", of: (task) => task.status }, event, (status) => ({ status }));
    const choosePriority = (event) => choose({ what: "priority", of: (task) => task.priority || defaultPriority(priorities.value) }, event, (value) => ({ priority: value }));
    function writeList(what, of, values, patch) {
      const field = { what, of: (task) => JSON.stringify(of(task)) };
      return writeField(field, field.of(props.task), JSON.stringify(values), patch);
    }
    const saveLabels = (labels2) => writeList("labels", (task) => task.labels ?? [], labels2, { labels: labels2 });
    const saveDependencies = (depends_on) => writeList("dependencies", (task) => task.depends_on ?? [], depends_on, { depends_on });
    async function appendNote(text) {
      try {
        await addNote(props.task.id, text);
        await refresh();
        toast(`Note added to ${props.task.id}`, "info");
        return true;
      } catch (error) {
        toast(`Could not add a note to ${props.task.id}: ${describe(error)}`);
        return false;
      }
    }
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("dialog", {
        id: "task-dialog",
        ref_key: "dialog",
        ref: dialog,
        class: "dialog task-dialog",
        onClose: _cache[6] || (_cache[6] = ($event) => emit2("close")),
        onMousedown: onMouseDown,
        onClick
      }, [
        createBaseVNode("div", _hoisted_113, [
          createBaseVNode("header", _hoisted_29, [
            createBaseVNode("div", _hoisted_38, [
              createBaseVNode("span", _hoisted_46, toDisplayString(__props.task.id), 1),
              createVNode(InlineText_default2, {
                id: "task-title",
                heading: "",
                label: "Title",
                value: __props.task.title,
                commit: saveTitle,
                onOpen: _cache[0] || (_cache[0] = ($event) => begin(TITLE)),
                onClose: _cache[1] || (_cache[1] = ($event) => editing.value = null)
              }, null, 8, ["value"]),
              createBaseVNode("select", {
                id: "task-status",
                class: "task-status",
                "aria-label": "Status",
                value: __props.task.status,
                onChange: chooseStatus
              }, [
                (openBlock(true), createElementBlock(Fragment, null, renderList(unref(columns), (column) => {
                  return openBlock(), createElementBlock("option", {
                    key: column.name,
                    value: column.name
                  }, toDisplayString(column.display_name), 9, _hoisted_64);
                }), 128))
              ], 40, _hoisted_54)
            ]),
            createBaseVNode("button", {
              type: "button",
              class: "ghost close",
              "data-close": "task-dialog",
              "aria-label": "Close",
              onClick: dismiss
            }, " ✕ ")
          ]),
          createBaseVNode("p", {
            id: "task-gone",
            class: "dialog-note gone",
            hidden: !unref(openTaskMissing)
          }, toDisplayString(__props.task.id) + " is no longer in the queue — it may have been deleted. Copy anything you still need here: a write has nothing left to land on. ", 9, _hoisted_73),
          createBaseVNode("p", {
            id: "task-changed",
            class: "dialog-note moved",
            hidden: moved.value === ""
          }, " The " + toDisplayString(moved.value) + " of " + toDisplayString(__props.task.id) + " changed on disk while you were editing it. Nothing here has been written — copy what you need, press Escape to see the file's version, and check the change in your VCS. ", 9, _hoisted_82),
          createBaseVNode("div", _hoisted_92, [
            createBaseVNode("section", _hoisted_10, [
              _cache[7] || (_cache[7] = createBaseVNode("h3", { class: "task-section" }, "Description", -1)),
              createVNode(InlineText_default2, {
                id: "task-body",
                multiline: "",
                label: "Description",
                value: split.value.content,
                commit: saveContent,
                onOpen: _cache[2] || (_cache[2] = ($event) => begin(DESCRIPTION)),
                onClose: _cache[3] || (_cache[3] = ($event) => editing.value = null)
              }, {
                default: withCtx(() => [
                  split.value.content !== "" ? (openBlock(), createBlock(Markdown_default2, {
                    key: 0,
                    source: split.value.content
                  }, null, 8, ["source"])) : (openBlock(), createElementBlock("p", _hoisted_11, "No description yet — click here to write one."))
                ]),
                _: 1
              }, 8, ["value"]),
              _cache[8] || (_cache[8] = createBaseVNode("h3", { class: "task-section" }, "Depends on", -1)),
              createVNode(TokenField_default2, {
                id: "task-depends-on",
                label: "a dependency",
                placeholder: "TQ-0002",
                values: __props.task.depends_on ?? [],
                commit: saveDependencies
              }, {
                default: withCtx(({ value }) => [
                  createBaseVNode("span", _hoisted_122, toDisplayString(value), 1)
                ]),
                _: 1
              }, 8, ["values"]),
              createBaseVNode("p", {
                id: "task-blocked",
                class: "blocked-note",
                hidden: pending.value.length === 0
              }, " Blocked by " + toDisplayString(pending.value.join(", ")), 9, _hoisted_132),
              createVNode(NotesPanel_default2, {
                notes: split.value.notes,
                commit: saveNote,
                append: appendNote
              }, null, 8, ["notes"])
            ]),
            createBaseVNode("aside", _hoisted_142, [
              _cache[9] || (_cache[9] = createBaseVNode("h3", { class: "task-section" }, "Priority", -1)),
              createBaseVNode("select", {
                id: "task-priority",
                "aria-label": "Priority",
                value: priority.value,
                onChange: choosePriority
              }, [
                (openBlock(true), createElementBlock(Fragment, null, renderList(priorityChoices.value, (option) => {
                  return openBlock(), createElementBlock("option", {
                    key: option.name,
                    value: option.name,
                    title: option.configured ? option.name : `${option.name} — not in the project's priority set`
                  }, toDisplayString(option.display), 9, _hoisted_162);
                }), 128))
              ], 40, _hoisted_152),
              _cache[10] || (_cache[10] = createBaseVNode("h3", { class: "task-section" }, "Assignee", -1)),
              createVNode(InlineText_default2, {
                id: "task-assignee",
                label: "Assignee",
                placeholder: "Unassigned",
                value: __props.task.assignee ?? "",
                commit: saveAssignee,
                onOpen: _cache[4] || (_cache[4] = ($event) => begin(ASSIGNEE)),
                onClose: _cache[5] || (_cache[5] = ($event) => editing.value = null)
              }, null, 8, ["value"]),
              _cache[11] || (_cache[11] = createBaseVNode("h3", { class: "task-section" }, "Labels", -1)),
              createVNode(TokenField_default2, {
                id: "task-labels",
                label: "a label",
                placeholder: "backend",
                values: __props.task.labels ?? [],
                commit: saveLabels
              }, {
                default: withCtx(({ value }) => [
                  createVNode(LabelChip_default2, { name: value }, null, 8, ["name"])
                ]),
                _: 1
              }, 8, ["values"]),
              createBaseVNode("p", _hoisted_172, toDisplayString(timestamps.value), 1)
            ])
          ])
        ])
      ], 544);
    };
  }
});

// frontend/components/TaskDialog.vue
var TaskDialog_default2 = TaskDialog_default;

// frontend/components/Toasts.vue?type=script
var _hoisted_114 = {
  id: "toasts",
  class: "toasts",
  "aria-live": "polite"
};
var Toasts_default = /* @__PURE__ */ defineComponent({
  __name: "Toasts",
  setup(__props) {
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock("div", _hoisted_114, [
        (openBlock(true), createElementBlock(Fragment, null, renderList(unref(toasts), (item2) => {
          return openBlock(), createElementBlock("div", {
            key: item2.id,
            class: normalizeClass(["toast", item2.kind])
          }, toDisplayString(item2.message), 3);
        }), 128))
      ]);
    };
  }
});

// frontend/components/Toasts.vue
var Toasts_default2 = Toasts_default;

// frontend/components/App.vue?type=script
var _hoisted_115 = { class: "topbar" };
var _hoisted_210 = { class: "statusbar" };
var _hoisted_39 = { id: "status-line" };
var App_default = /* @__PURE__ */ defineComponent({
  __name: "App",
  setup(__props) {
    return (_ctx, _cache) => {
      return openBlock(), createElementBlock(Fragment, null, [
        createBaseVNode("header", _hoisted_115, [
          _cache[3] || (_cache[3] = createBaseVNode("h1", { class: "brand" }, "tq", -1)),
          createVNode(SearchBar_default2),
          createBaseVNode("button", {
            id: "new-task",
            type: "button",
            class: "primary",
            onClick: _cache[0] || (_cache[0] = ($event) => creating.value = true)
          }, "New task")
        ]),
        createVNode(Board_default2),
        createBaseVNode("footer", _hoisted_210, [
          createBaseVNode("span", _hoisted_39, toDisplayString(unref(statusLine)), 1)
        ]),
        createVNode(Toasts_default2),
        createCommentVNode(` Keyed by the task: \`openTask\` is a ref rather than a find, so it could be
       pointed straight at another task without passing through nothing, and an
       instance reused across that switch would keep the first task's fields and
       save them onto the second. `),
        unref(openTask) ? (openBlock(), createBlock(TaskDialog_default2, {
          key: unref(openTask).id,
          task: unref(openTask),
          onClose: _cache[1] || (_cache[1] = ($event) => openTaskID.value = null)
        }, null, 8, ["task"])) : createCommentVNode("v-if", true),
        unref(creating) ? (openBlock(), createBlock(CreateDialog_default2, {
          key: 1,
          onClose: _cache[2] || (_cache[2] = ($event) => creating.value = false)
        })) : createCommentVNode("v-if", true)
      ], 64);
    };
  }
});

// frontend/components/App.vue
var App_default2 = App_default;

// frontend/main.ts
createApp(App_default2).mount("#app");
start();
