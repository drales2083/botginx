/* antibot preview API — the deliverable.
 *
 * Your panel builds its own form. This only answers two questions:
 *   what combinations exist?   →  catalog.supports(template)
 *   what should I display?     →  catalog.srcFor({...})
 *
 * No form, no styling, no framework. Plain ES5 on window, so it works from a
 * Go-rendered page, a React component, or a <script> tag in a static panel.
 * The browser receives images only — no template HTML, CSS or JS.
 *
 *   AntibotPreview.load('/preview/').then(function (catalog) {
 *     catalog.templates();                        // ['cloudflare', 'foyer', …]
 *     catalog.supports('latch');                  // {theme, puzzleMode, states, modes}
 *     img.src = catalog.srcFor({ template: 'latch', state: 'captcha',
 *                                theme: 'dark', mode: 'rotate' });
 *   });
 *
 * Or let it own an element:
 *   var view = AntibotPreview.mount(el, { base: '/preview/' });
 *   view.set({ template: 'latch', theme: 'dark' });
 */
(function (global) {
  'use strict';

  function unique(list) {
    return list.filter(function (v, i) { return list.indexOf(v) === i; });
  }

  function buildCatalog(rows, base) {
    function forTemplate(t) {
      return rows.filter(function (r) { return r.template === t; });
    }

    return {
      /** Every template that has captures, in manifest order. */
      templates: function () {
        return unique(rows.map(function (r) { return r.template; }));
      },

      /**
       * What a template actually offers. Derived from the captures that exist,
       * so a panel never has to hardcode which templates support theming or a
       * puzzle type — that list has gone stale before.
       */
      supports: function (template) {
        var rs = forTemplate(template);
        var themes = unique(rs.map(function (r) { return r.theme; }));
        var modes  = unique(rs.map(function (r) { return r.mode;  }));
        var states = unique(rs.map(function (r) { return r.state; }));
        return {
          theme: themes.length > 1,
          puzzleMode: modes.length > 1,
          themes: themes,
          modes: modes,
          states: states
        };
      },

      /** Dimensions every capture shares, for sizing a frame up front. */
      size: function () {
        return rows.length ? { w: rows[0].w, h: rows[0].h } : { w: 0, h: 0 };
      },

      /**
       * The image URL for a combination. Falls back within the template rather
       * than returning nothing: a panel may ask for a puzzle type on a state
       * that never draws a puzzle, and that should still show something.
       * Returns null only when the template is unknown.
       */
      srcFor: function (opts) {
        opts = opts || {};
        var rs = forTemplate(opts.template);
        if (!rs.length) return null;

        function match(fields) {
          return rs.filter(function (r) {
            for (var k in fields) if (fields[k] && r[k] !== fields[k]) return false;
            return true;
          })[0];
        }
        var hit = match({ state: opts.state, theme: opts.theme, mode: opts.mode })
               || match({ state: opts.state, theme: opts.theme })
               || match({ state: opts.state })
               || rs[0];
        return base + hit.src;
      },

      /** The raw rows, for anything the helpers above do not cover. */
      rows: function () { return rows.slice(); }
    };
  }

  var api = {
    /** Fetch the manifest once. base is the directory the images live under. */
    load: function (base) {
      base = base || '';
      if (base && base.charAt(base.length - 1) !== '/') base += '/';
      return fetch(base + 'images.json')
        .then(function (r) {
          if (!r.ok) throw new Error('preview manifest ' + r.status);
          return r.json();
        })
        .then(function (rows) { return buildCatalog(rows, base); });
    },

    /**
     * Convenience: own an <img> inside el and swap it on set(). Everything it
     * does is achievable with load() + srcFor() if the panel would rather
     * manage its own element.
     */
    mount: function (el, opts) {
      opts = opts || {};
      var img = document.createElement('img');
      img.alt = opts.alt || 'Challenge template preview';
      img.draggable = false;
      img.style.cssText = 'display:none;width:100%;height:100%;object-fit:contain';
      el.appendChild(img);

      var catalog = null, queued = null, current = {};

      function paint() {
        if (!catalog) return;
        var src = catalog.srcFor(current);
        console.log('[Preview API] paint() - current:', JSON.stringify(current), 'src:', src);
        if (!src) return;
        if (opts.onLoading) opts.onLoading(true);
        img.onload  = function () {
          img.style.display = 'block';
          if (opts.onLoading) opts.onLoading(false);
        };
        img.onerror = function () {
          img.style.display = 'none';
          if (opts.onLoading) opts.onLoading(false);
          if (opts.onMissing) opts.onMissing(src);
        };
        console.log('[Preview API] Setting img.src to:', src);
        img.src = src;
      }

      var ready = api.load(opts.base).then(function (c) {
        catalog = c;
        var size = c.size();
        if (opts.sizeFrame !== false && size.w) {
          el.style.aspectRatio = size.w + ' / ' + size.h;
        }
        if (queued) { current = queued; queued = null; }
        paint();
        return c;
      });

      return {
        ready: ready,
        catalog: function () { return catalog; },
        /** Merge fields into the current selection and repaint. */
        set: function (next) {
          console.log('[Preview API] set() called with:', next, 'current before merge:', JSON.stringify(current));
          for (var k in next) current[k] = next[k];
          console.log('[Preview API] current after merge:', JSON.stringify(current), 'catalog:', !!catalog);
          if (!catalog) { queued = current; return; }
          paint();
        },
        /** Warm every capture for a template so switching is instant. */
        preload: function (template) {
          if (!catalog) return;
          catalog.rows().forEach(function (r) {
            if (r.template === template) { var i = new Image(); i.src = opts.base + r.src; }
          });
        },
        destroy: function () { img.onload = img.onerror = null; el.removeChild(img); }
      };
    }
  };

  global.AntibotPreview = api;
})(window);
