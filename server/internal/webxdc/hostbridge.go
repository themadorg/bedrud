package webxdc

// HostBridgeJS is the trusted webxdc.js served instead of any ZIP entry.
//
// Design lessons from Delta Chat Desktop's webxdc-preload (re-implemented for
// postMessage, not copied): identity is applied before apps rely on the API;
// setUpdateListener pulls catch-up then resolves its Promise; status delivery
// is re-entrant (queued while a pull is in flight); sendUpdate is fire-and-forget
// and peer/own updates arrive via pull after a parent nudge.
const HostBridgeJS = `
(function () {
  "use strict";

  // ── WebRTC kill-switch (must run first, before app code) ─────────────────
  // Browser hosts cannot match Desktop's process-wide WebRTC blackhole, but
  // webxdc-test's webrtc-sidechannel (STUN srflx → public IP) is blocked by:
  // 1) CSP webrtc 'block' (Chromium), 2) defaulting constructors to Dead PC.
  //
  // IMPORTANT (OpenArena / Quake webxdc): apps install a pure-JS
  // FakeRTCPeerConnection that tunnels RTCDataChannel over
  // webxdc.joinRealtimeChannel() (see override-webrtc.js). That assignment
  // MUST stick — never use a no-op setter. We only reject restoring *native*
  // RTCPeerConnection. RTCSessionDescription / RTCIceCandidate stay available
  // as data bags (they do not open network by themselves).
  (function blockWebRTCSidechannel() {
    function deadPC() {
      throw new DOMException(
        "WebRTC is disabled in this WebXDC host (privacy). Use webxdc.joinRealtimeChannel() for multiplayer.",
        "NotAllowedError"
      );
    }
    function DeadPeerConnection() {
      deadPC();
    }
    DeadPeerConnection.prototype = {};
    DeadPeerConnection.generateCertificate = function () {
      return Promise.reject(new DOMException("WebRTC disabled", "NotAllowedError"));
    };

    function isNativePeerConnection(fn, nativePC) {
      if (typeof fn !== "function") return false;
      if (nativePC && fn === nativePC) return true;
      // Heuristic: browser native constructors usually have a non-JS [[NativeCode]] toString.
      try {
        var s = Function.prototype.toString.call(fn);
        if (s && s.indexOf("[native code]") !== -1) return true;
      } catch (e) {}
      return false;
    }

    function installPCTrap(win, propName, nativePC) {
      var current = DeadPeerConnection;
      try {
        Object.defineProperty(win, propName, {
          configurable: true,
          enumerable: false,
          get: function () {
            return current;
          },
          set: function (v) {
            // OpenArena does:
            //   RTCPeerConnection = FakeRTCPeerConnection           // function
            //   RTCPeerConnection = new Proxy(RTCPeerConnection, …) // typeof 'object'!
            // Reject only *native* constructors (STUN sidechannel). Allow Fake + Proxy.
            if (v == null) return;
            if (typeof v === "function") {
              if (isNativePeerConnection(v, nativePC)) return;
              current = v;
              return;
            }
            if (typeof v === "object") {
              // Proxy / polyfill instance — never the native function.
              current = v;
            }
          },
        });
      } catch (e1) {
        try {
          win[propName] = DeadPeerConnection;
        } catch (e2) {}
      }
    }

    function neuter(win) {
      if (!win) return;
      try {
        if (win.__bedrudWebrtcBlocked) return;
        win.__bedrudWebrtcBlocked = true;
      } catch (e) {
        return;
      }
      var nativePC = null;
      try {
        nativePC = win.RTCPeerConnection || win.webkitRTCPeerConnection || win.mozRTCPeerConnection || null;
      } catch (e0) {
        nativePC = null;
      }
      installPCTrap(win, "RTCPeerConnection", nativePC);
      installPCTrap(win, "webkitRTCPeerConnection", nativePC);
      installPCTrap(win, "mozRTCPeerConnection", nativePC);
      // RTCIceGatherer is rare and network-facing — always dead.
      try {
        Object.defineProperty(win, "RTCIceGatherer", {
          configurable: true,
          enumerable: false,
          get: function () {
            return DeadPeerConnection;
          },
          set: function () {},
        });
      } catch (eIG) {}
      // Freeze media capture that often pairs with WebRTC.
      try {
        if (win.navigator && win.navigator.mediaDevices) {
          var md = win.navigator.mediaDevices;
          var rej = function () {
            return Promise.reject(new DOMException("Media blocked", "NotAllowedError"));
          };
          try {
            Object.defineProperty(md, "getUserMedia", { value: rej, configurable: true });
          } catch (e3) {
            md.getUserMedia = rej;
          }
          try {
            Object.defineProperty(md, "getDisplayMedia", { value: rej, configurable: true });
          } catch (e4) {
            if (md.getDisplayMedia) md.getDisplayMedia = rej;
          }
        }
        if (win.navigator) {
          win.navigator.getUserMedia = function (c, s, err) {
            if (err) err(new DOMException("Media blocked", "NotAllowedError"));
          };
          win.navigator.webkitGetUserMedia = win.navigator.getUserMedia;
          win.navigator.mozGetUserMedia = win.navigator.getUserMedia;
        }
      } catch (e5) {}
    }

    neuter(window);

    function hookIframe(el) {
      if (!el || el.nodeType !== 1) return;
      try {
        if (String(el.tagName).toUpperCase() !== "IFRAME") return;
      } catch (e) {
        return;
      }
      var patch = function () {
        try {
          if (el.contentWindow) neuter(el.contentWindow);
        } catch (e) {}
      };
      // about:blank is available immediately (webxdc-test "uninitialized" probe).
      // Must be synchronous — MutationObserver is too late for same-tick reads.
      try {
        patch();
      } catch (e) {}
      if (!el.__bedrudWebrtcLoadHook) {
        el.__bedrudWebrtcLoadHook = true;
        try {
          el.addEventListener("load", patch, true);
        } catch (e) {}
      }
    }

    function scanNode(n) {
      if (!n || n.nodeType !== 1) return;
      if (n.tagName === "IFRAME") hookIframe(n);
      try {
        var list = n.getElementsByTagName && n.getElementsByTagName("iframe");
        if (list) {
          for (var i = 0; i < list.length; i++) hookIframe(list[i]);
        }
      } catch (e) {}
    }

    // createElement / createElementNS
    try {
      var ce = Document.prototype.createElement;
      Document.prototype.createElement = function (tagName) {
        var el = ce.apply(this, arguments);
        try {
          if (String(tagName).toLowerCase() === "iframe") hookIframe(el);
        } catch (e) {}
        return el;
      };
    } catch (e) {}
    try {
      var ceNS = Document.prototype.createElementNS;
      Document.prototype.createElementNS = function (ns, tagName) {
        var el = ceNS.apply(this, arguments);
        try {
          var local = String(tagName).toLowerCase();
          if (local === "iframe" || local.slice(-6) === ":iframe") hookIframe(el);
        } catch (e) {}
        return el;
      };
    } catch (e) {}

    // Synchronous DOM hooks — webxdc-test does innerHTML += "<iframe>" then
    // immediately reads contentWindow.RTCPeerConnection (MutationObserver is async).
    function wrapTreeMethod(proto, name) {
      try {
        var orig = proto[name];
        if (typeof orig !== "function") return;
        proto[name] = function () {
          var r = orig.apply(this, arguments);
          try {
            for (var i = 0; i < arguments.length; i++) scanNode(arguments[i]);
            if (this && this.nodeType === 1) scanNode(this);
          } catch (e) {}
          return r;
        };
      } catch (e) {}
    }
    try {
      wrapTreeMethod(Node.prototype, "appendChild");
      wrapTreeMethod(Node.prototype, "insertBefore");
      wrapTreeMethod(Node.prototype, "replaceChild");
      wrapTreeMethod(Element.prototype, "append");
      wrapTreeMethod(Element.prototype, "prepend");
      wrapTreeMethod(Element.prototype, "before");
      wrapTreeMethod(Element.prototype, "after");
      wrapTreeMethod(Element.prototype, "replaceWith");
    } catch (e) {}

    try {
      var ih = Object.getOwnPropertyDescriptor(Element.prototype, "innerHTML");
      if (ih && ih.set && ih.get) {
        Object.defineProperty(Element.prototype, "innerHTML", {
          configurable: true,
          enumerable: ih.enumerable,
          get: function () {
            return ih.get.call(this);
          },
          set: function (v) {
            ih.set.call(this, v);
            try {
              scanNode(this);
            } catch (e) {}
          },
        });
      }
    } catch (e) {}
    try {
      var ohtml = Object.getOwnPropertyDescriptor(Element.prototype, "outerHTML");
      if (ohtml && ohtml.set && ohtml.get) {
        Object.defineProperty(Element.prototype, "outerHTML", {
          configurable: true,
          enumerable: ohtml.enumerable,
          get: function () {
            return ohtml.get.call(this);
          },
          set: function (v) {
            var parent = this.parentNode;
            ohtml.set.call(this, v);
            try {
              if (parent) scanNode(parent);
            } catch (e) {}
          },
        });
      }
    } catch (e) {}
    try {
      var iah = Element.prototype.insertAdjacentHTML;
      if (iah) {
        Element.prototype.insertAdjacentHTML = function (pos, html) {
          var r = iah.call(this, pos, html);
          try {
            scanNode(this);
            if (this.parentNode) scanNode(this.parentNode);
          } catch (e) {}
          return r;
        };
      }
    } catch (e) {}

    try {
      var obs = new MutationObserver(function (muts) {
        for (var i = 0; i < muts.length; i++) {
          var nodes = muts[i].addedNodes;
          for (var j = 0; j < nodes.length; j++) scanNode(nodes[j]);
        }
      });
      var root = document.documentElement || document;
      obs.observe(root, { childList: true, subtree: true });
    } catch (e) {}

    // Re-neuter periodically (probes may re-grab native refs via fresh frames).
    try {
      setInterval(function () {
        neuter(window);
        try {
          var iframes = document.getElementsByTagName("iframe");
          for (var i = 0; i < iframes.length; i++) {
            hookIframe(iframes[i]);
          }
        } catch (e2) {}
      }, 100);
    } catch (e) {}
  })();

  var CHANNEL = "bedrud-webxdc";
  var appId = null;
  var initialized = false;
  var parentOrigin = "*";

  // Identity — Desktop starts with placeholders until setup/init.
  var selfAddr = "?Setup Missing?";
  var selfName = "?Setup Missing?";
  var selfAvatarUrl = "";
  var sendUpdateInterval = 10000;
  var sendUpdateMaxSize = 128000;

  /** @type {null|function} */
  var updateListener = null;
  var lastSerial = 0;
  /** @type {null|function} */
  var setUpdateListenerResolve = null;
  var pullRunning = false;
  var pullScheduled = false;
  var pullSeq = 0;
  /** @type {Object.<string, function>} */
  var pendingPulls = {};

  var realtimeListener = null;
  var pendingChat = {};
  var chatReqSeq = 0;
  var readyTimer = null;

  function post(msg) {
    try {
      var payload = Object.assign({ channel: CHANNEL }, msg);
      // Always stamp appId so parent never drops rtJoin/rtSend after in-app navigations.
      if (appId) payload.appId = appId;
      // Prefer * until init pins parentOrigin — specific origins can race SPA localhost vs 127.0.0.1.
      var target = parentOrigin && parentOrigin !== "*" ? parentOrigin : "*";
      parent.postMessage(payload, target);
    } catch (e) {}
  }

  function applyIdentity(d) {
    if (typeof d.selfAddr === "string" && d.selfAddr) selfAddr = d.selfAddr;
    if (typeof d.selfName === "string" && d.selfName) selfName = d.selfName;
    if (typeof d.selfAvatarUrl === "string") selfAvatarUrl = d.selfAvatarUrl;
    if (typeof d.sendUpdateInterval === "number" && d.sendUpdateInterval > 0) {
      sendUpdateInterval = d.sendUpdateInterval;
    }
    if (typeof d.sendUpdateMaxSize === "number" && d.sendUpdateMaxSize > 0) {
      sendUpdateMaxSize = d.sendUpdateMaxSize;
    }
    // Keep plain props in sync (some apps copy them once; getters below are preferred).
    try {
      api.selfAddr = selfAddr;
      api.selfName = selfName;
      api.sendUpdateInterval = sendUpdateInterval;
      api.sendUpdateMaxSize = sendUpdateMaxSize;
    } catch (e) {}
  }

  function stopReadyPing() {
    if (readyTimer != null) {
      clearInterval(readyTimer);
      readyTimer = null;
    }
  }

  function startReadyPing() {
    stopReadyPing();
    function ping() {
      if (initialized) {
        stopReadyPing();
        return;
      }
      post({ type: "ready" });
    }
    ping();
    readyTimer = setInterval(ping, 400);
  }

  /**
   * Pull updates after lastSerial and deliver to listener (Desktop innerOnStatusUpdate).
   * Resolves setUpdateListener Promise after the first successful catch-up.
   */
  /**
   * Deliver one or more status updates to the app listener (Desktop innerOnStatusUpdate).
   * Dedupes by serial so statusPush + pull-nudge cannot double-fire.
   */
  function deliverUpdates(list) {
    var updates = list || [];
    var delivered = [];
    for (var i = 0; i < updates.length; i++) {
      var u = updates[i];
      if (!u || typeof u !== "object") continue;
      var ser = typeof u.serial === "number" ? u.serial : 0;
      // Skip already-seen serials (local statusPush + later GET can race).
      if (ser > 0 && ser <= lastSerial) continue;
      // Desktop advances last_serial from max_serial on each delivered update.
      if (typeof u.max_serial === "number" && u.max_serial > lastSerial) {
        lastSerial = u.max_serial;
      } else if (ser > lastSerial) {
        lastSerial = ser;
      }
      if (typeof updateListener === "function") {
        try { updateListener(u); } catch (e) {}
      }
      delivered.push(u);
    }
    return delivered;
  }

  function pullUpdates() {
    return new Promise(function (resolve) {
      var reqId = "u-" + (++pullSeq);
      var settled = false;
      function finish(list) {
        if (settled) return;
        settled = true;
        delete pendingPulls[reqId];
        var delivered = deliverUpdates(list);
        if (setUpdateListenerResolve) {
          var r = setUpdateListenerResolve;
          setUpdateListenerResolve = null;
          try { r(); } catch (e) {}
        }
        resolve(delivered);
      }
      pendingPulls[reqId] = finish;
      post({ type: "getUpdates", requestId: reqId, after: lastSerial });
      setTimeout(function () {
        if (pendingPulls[reqId]) finish([]);
      }, 12000);
    });
  }

  /** Re-entrant pull scheduler (Desktop onStatusUpdate). */
  function onStatusNudge() {
    if (pullRunning) {
      pullScheduled = true;
      return;
    }
    pullRunning = true;
    pullUpdates().then(function () {
      if (pullScheduled) {
        pullScheduled = false;
        pullRunning = false;
        onStatusNudge();
      } else {
        pullRunning = false;
      }
    });
  }

  function RealtimeListener() {
    this.listener = null;
    this.trashed = false;
  }
  RealtimeListener.prototype.setListener = function (listener) {
    this.listener = typeof listener === "function" ? listener : null;
  };
  RealtimeListener.prototype.send = function (data) {
    // Accept Uint8Array, ArrayBuffer, or any ArrayBufferView (OpenArena packets).
    var bytes;
    if (data instanceof Uint8Array) {
      bytes = data;
    } else if (typeof ArrayBuffer !== "undefined" && data instanceof ArrayBuffer) {
      bytes = new Uint8Array(data);
    } else if (data && typeof data === "object" && typeof data.byteLength === "number" && data.buffer) {
      try {
        bytes = new Uint8Array(data.buffer, data.byteOffset || 0, data.byteLength);
      } catch (e0) {
        throw new Error("realtime listener data must be a Uint8Array");
      }
    } else {
      throw new Error("realtime listener data must be a Uint8Array");
    }
    if (this.trashed) {
      throw new Error("realtime listener is trashed and can no longer be used");
    }
    if (bytes.byteLength > 128000) {
      throw new Error("realtime data exceeds 128000 bytes");
    }
    // Array.from preserves bytes better than slice.call on some engines.
    var arr = [];
    for (var i = 0; i < bytes.length; i++) arr.push(bytes[i]);
    post({ type: "rtSend", data: arr });
  };
  RealtimeListener.prototype.leave = function () {
    this.trashed = true;
    this.listener = null;
    post({ type: "rtLeave" });
    if (realtimeListener === this) realtimeListener = null;
  };
  RealtimeListener.prototype.is_trashed = function () {
    return this.trashed;
  };
  RealtimeListener.prototype.receive = function (data) {
    if (this.trashed) {
      throw new Error("realtime listener is trashed and can no longer be used");
    }
    if (this.listener) {
      try { this.listener(data); } catch (e) {}
    }
  };

  window.addEventListener("message", function (ev) {
    var d = ev.data;
    if (!d || d.channel !== CHANNEL) return;

    if (d.type === "init") {
      if (ev.origin && ev.origin !== "null") parentOrigin = ev.origin;
      if (typeof d.appId === "string" && d.appId) appId = d.appId;
      applyIdentity(d);
      initialized = true;
      stopReadyPing();
      // If app already registered a listener before init completed, catch up now.
      if (updateListener) onStatusNudge();
      // Re-assert realtime join after parent is ready. OpenArena calls
      // joinRealtimeChannel in a module script that often races the parent
      // message listener; without this re-post, WHO_IS never leaves the iframe.
      if (realtimeListener && !realtimeListener.is_trashed()) {
        post({ type: "rtJoin" });
      }
      return;
    }

    if (d.type === "updates" && d.requestId != null) {
      var fin = pendingPulls[d.requestId];
      if (fin) fin(Array.isArray(d.updates) ? d.updates : []);
      return;
    }

    // Immediate local/peer push (Desktop core delivers own updates; we push after POST).
    // Avoids "current update missing" when pull races rate-limit or LiveKit.
    if (d.type === "statusPush" && d.update && typeof d.update === "object") {
      deliverUpdates([d.update]);
      return;
    }

    // Parent nudge: new status available — pull (Desktop webxdc.statusUpdate).
    if (d.type === "statusNudge" || d.type === "statusUpdate") {
      // If a full update body is included, deliver it then still pull for catch-up.
      if (d.update && typeof d.update === "object") {
        deliverUpdates([d.update]);
      }
      onStatusNudge();
      return;
    }

    if (d.type === "rtData" && realtimeListener && !realtimeListener.is_trashed()) {
      try {
        realtimeListener.receive(Uint8Array.from(d.data || []));
      } catch (e) {}
      return;
    }

    if (d.type === "sendToChatResult" && d.requestId != null) {
      var p = pendingChat[d.requestId];
      if (!p) return;
      delete pendingChat[d.requestId];
      if (d.ok) p.resolve();
      else p.reject(new Error(d.error || "sendToChat failed"));
      return;
    }
  });

  // External http(s) — confirm in parent (spec 1.3).
  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    var a = t.closest("a");
    if (!a) return;
    var href = a.getAttribute("href");
    if (!href) return;
    var lower = href.toLowerCase();
    if (lower.indexOf("http:") === 0 || lower.indexOf("https:") === 0 || href.indexOf("//") === 0) {
      e.preventDefault();
      e.stopPropagation();
      post({ type: "openExternal", url: href });
    }
  }, true);

  function blobToBase64(blob) {
    return new Promise(function (resolve, reject) {
      var reader = new FileReader();
      reader.onload = function () {
        var data = String(reader.result || "");
        var marker = ";base64,";
        var i = data.indexOf(marker);
        resolve(i >= 0 ? data.slice(i + marker.length) : data);
      };
      reader.onerror = function () { reject(reader.error || new Error("read failed")); };
      reader.readAsDataURL(blob);
    });
  }

  var api = {
    selfAddr: selfAddr,
    selfName: selfName,
    sendUpdateInterval: sendUpdateInterval,
    sendUpdateMaxSize: sendUpdateMaxSize,

    sendUpdate: function (update, description) {
      if (description) {
        try {
          console.error("parameter description in sendUpdate is deprecated and will be removed in the future");
        } catch (e) {}
      }
      post({ type: "sendUpdate", update: update });
    },

    setUpdateListener: function (cb, start_serial) {
      lastSerial = typeof start_serial === "number" && isFinite(start_serial) ? start_serial : 0;
      updateListener = typeof cb === "function" ? cb : null;
      var promise = new Promise(function (resolve) {
        setUpdateListenerResolve = resolve;
      });
      // Pull catch-up then resolve (Desktop onStatusUpdate after setUpdateListener).
      onStatusNudge();
      return promise;
    },

    getAllUpdates: function () {
      try {
        console.error(
          "getAllUpdates is deprecated and will be removed in the future, it also returns an empty array now, so you really should use setUpdateListener instead."
        );
      } catch (e) {}
      return Promise.resolve([]);
    },

    sendToChat: function (content) {
      if (!content || (!content.file && !content.text)) {
        return Promise.reject(
          "Error from sendToChat: Invalid empty message, at least one of text or file should be provided"
        );
      }
      return Promise.resolve().then(function () {
        if (!content.file) {
          return { text: String(content.text || ""), file: null };
        }
        var f = content.file;
        if (!f.name) return Promise.reject("file name is missing");
        var modes = ["blob", "base64", "plainText"].filter(function (k) {
          return Object.prototype.hasOwnProperty.call(f, k);
        });
        if (modes.length > 1) {
          return Promise.reject("you can only set one of blob, base64 or plainText, not multiple ones");
        }
        if (f.blob instanceof Blob) {
          return blobToBase64(f.blob).then(function (b64) {
            return {
              text: content.text != null ? String(content.text) : "",
              file: { name: f.name, base64: b64, mime: f.blob.type || "application/octet-stream" },
            };
          });
        }
        if (typeof f.base64 === "string") {
          return {
            text: content.text != null ? String(content.text) : "",
            file: { name: f.name, base64: f.base64, mime: "application/octet-stream" },
          };
        }
        if (typeof f.plainText === "string") {
          return blobToBase64(new Blob([f.plainText])).then(function (b64) {
            return {
              text: content.text != null ? String(content.text) : "",
              file: { name: f.name, base64: b64, mime: "text/plain" },
            };
          });
        }
        return Promise.reject(
          "data is not set or wrong format, set one of blob, base64 or plainText, see webxdc documentation for sendToChat"
        );
      }).then(function (payload) {
        var requestId = "stc-" + (++chatReqSeq);
        return new Promise(function (resolve, reject) {
          pendingChat[requestId] = { resolve: resolve, reject: reject };
          post({
            type: "sendToChat",
            requestId: requestId,
            text: payload.text,
            file: payload.file,
          });
          setTimeout(function () {
            if (pendingChat[requestId]) {
              delete pendingChat[requestId];
              reject(new Error("sendToChat timed out"));
            }
          }, 120000);
        });
      });
    },

    importFiles: function (filters) {
      filters = filters || {};
      var element = document.createElement("input");
      element.type = "file";
      element.accept = []
        .concat(filters.extensions || [])
        .concat(filters.mimeTypes || [])
        .join(",");
      element.multiple = filters.multiple || false;
      var promise = new Promise(function (resolve) {
        element.onchange = function () {
          var files = Array.prototype.slice.call(element.files || []);
          try { document.body.removeChild(element); } catch (e) {}
          resolve(files);
        };
      });
      element.style.display = "none";
      document.body.appendChild(element);
      element.click();
      return promise;
    },

    joinRealtimeChannel: function () {
      // OpenArena re-joins after location.replace(?serverAddress=). Fresh page
      // usually has no listener; if one exists (or leave failed), leave first.
      // Do NOT require window.top (cross-origin iframe cannot touch top).
      if (realtimeListener && !realtimeListener.is_trashed()) {
        try {
          realtimeListener.leave();
        } catch (eLeave) {
          realtimeListener = null;
        }
      }
      realtimeListener = new RealtimeListener();
      // Bind methods so destructuring works (Desktop binds explicitly).
      realtimeListener.setListener = realtimeListener.setListener.bind(realtimeListener);
      realtimeListener.send = realtimeListener.send.bind(realtimeListener);
      realtimeListener.leave = realtimeListener.leave.bind(realtimeListener);
      realtimeListener.is_trashed = realtimeListener.is_trashed.bind(realtimeListener);
      // Expose for apps that stash the channel (Desktop uses window.top; we use self).
      try {
        window.__webxdcRealtimeChannel = realtimeListener;
      } catch (eEx) {}
      post({ type: "rtJoin" });
      // Parent may not be listening yet (React effect attaches after iframe load).
      // Re-post a few times so OpenArena WHO_IS is not stuck with rtJoined=false.
      var joinRetries = 0;
      var joinRetryId = setInterval(function () {
        joinRetries++;
        if (!realtimeListener || realtimeListener.is_trashed() || joinRetries > 25) {
          clearInterval(joinRetryId);
          return;
        }
        post({ type: "rtJoin" });
      }, 200);
      return realtimeListener;
    },
  };

  // Persist appId across in-app navigations that drop query params (OpenArena
  // rewrites ?basegame= / ?serverAddress= and can lose appId, which breaks realtime).
  try {
    var APPID_KEY = "__bedrud_webxdc_appid__";
    var q = new URLSearchParams(location.search);
    var qApp = q.get("appId");
    if (qApp) {
      appId = qApp;
      try { sessionStorage.setItem(APPID_KEY, qApp); } catch (eS) {}
    } else {
      try {
        var stored = sessionStorage.getItem(APPID_KEY);
        if (stored) appId = stored;
      } catch (eS2) {}
    }
  } catch (e) {}

  /**
   * Cookie polyfill when third-party iframe policy blocks document.cookie.
   *
   * Official webxdc-test (js/cookies.js) does setCookie then getCookie; if the
   * value is missing it shows "WARNING: cookies are not supported."
   * In Bedrud the mini-app runs in a cross-origin iframe (SPA ≠ webxdc host).
   * Modern Chrome treats that as a third-party context and often refuses
   * document.cookie even with allow-same-origin. WebXDC itself only requires
   * localStorage / sessionStorage / IndexedDB — cookies are optional. We still
   * emulate them via localStorage so apps that probe cookies (and simple state)
   * work. Only installed when a real cookie write is rejected.
   */
  (function installCookieFallbackIfNeeded() {
    var LS_KEY = "__bedrud_webxdc_cookies__";
    function nativeCookiesWork() {
      try {
        var token = "__wx_ck_" + String(Math.random()).slice(2);
        document.cookie = token + "=1; path=/; max-age=60";
        var ok = document.cookie.indexOf(token + "=") !== -1;
        document.cookie = token + "=; path=/; max-age=0";
        return ok;
      } catch (e) {
        return false;
      }
    }
    if (nativeCookiesWork()) return;

    function loadJar() {
      try {
        return JSON.parse(localStorage.getItem(LS_KEY) || "{}") || {};
      } catch (e) {
        return {};
      }
    }
    function saveJar(jar) {
      try {
        localStorage.setItem(LS_KEY, JSON.stringify(jar));
      } catch (e) {}
    }
    function jarToCookieString(jar) {
      var now = Date.now();
      var parts = [];
      var changed = false;
      Object.keys(jar).forEach(function (k) {
        var e = jar[k];
        if (!e) return;
        if (typeof e.exp === "number" && e.exp > 0 && e.exp < now) {
          delete jar[k];
          changed = true;
          return;
        }
        parts.push(k + "=" + e.v);
      });
      if (changed) saveJar(jar);
      return parts.join("; ");
    }
    function applySetCookie(raw) {
      if (!raw) return;
      var segs = String(raw).split(";");
      var nv = segs[0].split("=");
      var name = (nv.shift() || "").trim();
      if (!name) return;
      var value = nv.join("=");
      var exp = 0;
      var maxAge = null;
      for (var i = 1; i < segs.length; i++) {
        var p = segs[i].trim();
        var eq = p.indexOf("=");
        var key = (eq >= 0 ? p.slice(0, eq) : p).trim().toLowerCase();
        var val = eq >= 0 ? p.slice(eq + 1).trim() : "";
        if (key === "max-age") {
          var sec = parseInt(val, 10);
          if (!isNaN(sec)) maxAge = sec;
        } else if (key === "expires") {
          var t = Date.parse(val);
          if (!isNaN(t)) exp = t;
        }
      }
      if (maxAge !== null) {
        exp = maxAge <= 0 ? 1 : Date.now() + maxAge * 1000;
      }
      var jar = loadJar();
      if (exp === 1 || (exp > 0 && exp < Date.now())) {
        delete jar[name];
      } else {
        jar[name] = { v: value, exp: exp || 0 };
      }
      saveJar(jar);
    }

    try {
      Object.defineProperty(Document.prototype, "cookie", {
        configurable: true,
        enumerable: true,
        get: function () {
          return jarToCookieString(loadJar());
        },
        set: function (v) {
          applySetCookie(v);
        },
      });
    } catch (e1) {
      try {
        Object.defineProperty(document, "cookie", {
          configurable: true,
          enumerable: true,
          get: function () {
            return jarToCookieString(loadJar());
          },
          set: function (v) {
            applySetCookie(v);
          },
        });
      } catch (e2) {}
    }
  })();

  // Live getters so late init always updates what apps read (OpenArena player name).
  try {
    Object.defineProperty(api, "selfAddr", {
      configurable: true,
      enumerable: true,
      get: function () { return selfAddr; },
      set: function (v) { if (typeof v === "string" && v) selfAddr = v; },
    });
    Object.defineProperty(api, "selfName", {
      configurable: true,
      enumerable: true,
      get: function () { return selfName; },
      set: function (v) { if (typeof v === "string" && v) selfName = v; },
    });
  } catch (eGet) {}

  window.webxdc = api;
  // Keep pinging ready until parent init (fixes race if parent attaches listener late).
  startReadyPing();
})();
`
