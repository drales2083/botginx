// pkg/customizer/loaders.go
package customizer

// loaderData holds HTML and CSS for each loader
type loaderData struct {
	name     string
	category string
	html     string
	css      string
}

// loaders contains all 110 loader definitions
var loaders = map[string]loaderData{
	// =============================================================================
	// SPINNERS & RINGS (23)
	// =============================================================================

	"spinner-classic": {
		name:     "Spinner Classic",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-classic"></div>`,
		css: `.spinner-classic{width:48px;height:48px;border:4px solid var(--secondary);border-top-color:var(--primary);border-radius:50%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-thick": {
		name:     "Spinner Thick",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-thick"></div>`,
		css: `.spinner-thick{width:48px;height:48px;border:6px solid var(--secondary);border-top-color:var(--primary);border-radius:50%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-thin": {
		name:     "Spinner Thin",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-thin"></div>`,
		css: `.spinner-thin{width:48px;height:48px;border:2px solid var(--secondary);border-top-color:var(--primary);border-radius:50%;animation:spin .8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-dual": {
		name:     "Spinner Dual",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-dual"></div>`,
		css: `.spinner-dual{width:48px;height:48px;border:4px solid transparent;border-top-color:var(--primary);border-bottom-color:var(--secondary);border-radius:50%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-triple": {
		name:     "Spinner Triple",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-triple"><span></span><span></span><span></span></div>`,
		css: `.spinner-triple{width:48px;height:48px;position:relative}
.spinner-triple span{position:absolute;inset:0;border:3px solid transparent;border-radius:50%}
.spinner-triple span:nth-child(1){border-top-color:var(--primary);animation:spin 1s linear infinite}
.spinner-triple span:nth-child(2){border-right-color:var(--secondary);animation:spin 1.5s linear infinite reverse}
.spinner-triple span:nth-child(3){border-bottom-color:var(--primary);animation:spin 2s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-dotted": {
		name:     "Spinner Dotted",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-dotted"></div>`,
		css: `.spinner-dotted{width:48px;height:48px;border:4px dotted var(--primary);border-radius:50%;animation:spin 1.5s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-dashed": {
		name:     "Spinner Dashed",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-dashed"></div>`,
		css: `.spinner-dashed{width:48px;height:48px;border:4px dashed var(--primary);border-radius:50%;animation:spin 2s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-gradient": {
		name:     "Spinner Gradient",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-gradient"></div>`,
		css: `.spinner-gradient{width:48px;height:48px;border-radius:50%;background:conic-gradient(var(--primary),transparent);animation:spin 1s linear infinite;-webkit-mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 4px));mask:radial-gradient(farthest-side,transparent calc(100% - 4px),#000 calc(100% - 4px))}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"spinner-glow": {
		name:     "Spinner Glow",
		category: "Spinners & Rings",
		html:     `<div class="loader spinner-glow"></div>`,
		css: `.spinner-glow{width:48px;height:48px;border:4px solid var(--secondary);border-top-color:var(--primary);border-radius:50%;animation:spin 1s linear infinite;box-shadow:0 0 15px var(--primary)}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-pulse": {
		name:     "Ring Pulse",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-pulse"></div>`,
		css: `.ring-pulse{width:48px;height:48px;border:4px solid var(--primary);border-radius:50%;animation:ring-pulse 1s ease-out infinite}
@keyframes ring-pulse{0%{transform:scale(.8);opacity:1}100%{transform:scale(1.2);opacity:0}}`,
	},
	"ring-ripple": {
		name:     "Ring Ripple",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-ripple"><span></span><span></span></div>`,
		css: `.ring-ripple{position:relative;width:48px;height:48px}
.ring-ripple span{position:absolute;inset:0;border:4px solid var(--primary);border-radius:50%;animation:ring-ripple 1.5s ease-out infinite}
.ring-ripple span:nth-child(2){animation-delay:.5s}
@keyframes ring-ripple{0%{transform:scale(.5);opacity:1}100%{transform:scale(1.2);opacity:0}}`,
	},
	"ring-orbit": {
		name:     "Ring Orbit",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-orbit"><span></span></div>`,
		css: `.ring-orbit{position:relative;width:48px;height:48px;border:2px solid var(--secondary);border-radius:50%}
.ring-orbit span{position:absolute;width:10px;height:10px;background:var(--primary);border-radius:50%;top:-5px;left:50%;margin-left:-5px;animation:orbit 1s linear infinite;transform-origin:5px 29px}
@keyframes orbit{to{transform:rotate(360deg)}}`,
	},
	"ring-chase": {
		name:     "Ring Chase",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-chase"><span></span><span></span><span></span></div>`,
		css: `.ring-chase{position:relative;width:48px;height:48px;animation:spin 2s linear infinite}
.ring-chase span{position:absolute;width:8px;height:8px;background:var(--primary);border-radius:50%}
.ring-chase span:nth-child(1){top:0;left:50%;margin-left:-4px}
.ring-chase span:nth-child(2){bottom:0;left:50%;margin-left:-4px}
.ring-chase span:nth-child(3){top:50%;left:0;margin-top:-4px}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-double": {
		name:     "Ring Double",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-double"><span></span><span></span></div>`,
		css: `.ring-double{position:relative;width:48px;height:48px}
.ring-double span{position:absolute;inset:0;border:3px solid transparent;border-radius:50%}
.ring-double span:nth-child(1){border-top-color:var(--primary);animation:spin 1s linear infinite}
.ring-double span:nth-child(2){inset:6px;border-bottom-color:var(--secondary);animation:spin 1s linear infinite reverse}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-zoom": {
		name:     "Ring Zoom",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-zoom"></div>`,
		css: `.ring-zoom{width:48px;height:48px;border:4px solid var(--primary);border-radius:50%;animation:ring-zoom 1s ease-in-out infinite}
@keyframes ring-zoom{0%,100%{transform:scale(1)}50%{transform:scale(.7)}}`,
	},
	"ring-neon": {
		name:     "Ring Neon",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-neon"></div>`,
		css: `.ring-neon{width:48px;height:48px;border:3px solid var(--primary);border-radius:50%;animation:spin 1s linear infinite,ring-neon-glow 1s ease-in-out infinite}
@keyframes spin{to{transform:rotate(360deg)}}
@keyframes ring-neon-glow{0%,100%{box-shadow:0 0 5px var(--primary),0 0 10px var(--primary)}50%{box-shadow:0 0 20px var(--primary),0 0 30px var(--primary)}}`,
	},
	"ring-loading": {
		name:     "Ring Loading",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-loading"></div>`,
		css: `.ring-loading{width:48px;height:48px;border:4px solid var(--secondary);border-radius:50%;position:relative;animation:ring-loading 1.5s linear infinite}
.ring-loading::before{content:'';position:absolute;inset:-4px;border:4px solid transparent;border-top-color:var(--primary);border-radius:50%}
@keyframes ring-loading{to{transform:rotate(360deg)}}`,
	},
	"ring-spinner-dots": {
		name:     "Ring Spinner Dots",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-spinner-dots"><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.ring-spinner-dots{position:relative;width:48px;height:48px;animation:spin 1.2s linear infinite}
.ring-spinner-dots span{position:absolute;width:6px;height:6px;background:var(--primary);border-radius:50%;transform-origin:24px 24px}
.ring-spinner-dots span:nth-child(1){transform:rotate(0deg) translateX(20px);opacity:1}
.ring-spinner-dots span:nth-child(2){transform:rotate(45deg) translateX(20px);opacity:.875}
.ring-spinner-dots span:nth-child(3){transform:rotate(90deg) translateX(20px);opacity:.75}
.ring-spinner-dots span:nth-child(4){transform:rotate(135deg) translateX(20px);opacity:.625}
.ring-spinner-dots span:nth-child(5){transform:rotate(180deg) translateX(20px);opacity:.5}
.ring-spinner-dots span:nth-child(6){transform:rotate(225deg) translateX(20px);opacity:.375}
.ring-spinner-dots span:nth-child(7){transform:rotate(270deg) translateX(20px);opacity:.25}
.ring-spinner-dots span:nth-child(8){transform:rotate(315deg) translateX(20px);opacity:.125}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-moon": {
		name:     "Ring Moon",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-moon"><span></span></div>`,
		css: `.ring-moon{width:48px;height:48px;position:relative}
.ring-moon::before{content:'';position:absolute;inset:0;border:3px solid var(--secondary);border-radius:50%}
.ring-moon span{position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;top:0;left:50%;margin-left:-6px;animation:orbit 1.2s linear infinite;transform-origin:6px 24px}`,
	},
	"ring-atom": {
		name:     "Ring Atom",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-atom"><span></span><span></span><span></span></div>`,
		css: `.ring-atom{position:relative;width:48px;height:48px}
.ring-atom::before{content:'';position:absolute;width:8px;height:8px;background:var(--primary);border-radius:50%;top:50%;left:50%;transform:translate(-50%,-50%)}
.ring-atom span{position:absolute;inset:0;border:2px solid var(--secondary);border-radius:50%;animation:spin 1.5s linear infinite}
.ring-atom span:nth-child(2){transform:rotate(60deg);animation-duration:1.8s}
.ring-atom span:nth-child(3){transform:rotate(120deg);animation-duration:2.1s}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-saturn": {
		name:     "Ring Saturn",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-saturn"><span></span></div>`,
		css: `.ring-saturn{position:relative;width:48px;height:48px}
.ring-saturn::before{content:'';position:absolute;width:20px;height:20px;background:var(--primary);border-radius:50%;top:50%;left:50%;transform:translate(-50%,-50%)}
.ring-saturn span{position:absolute;inset:0;border:3px solid var(--secondary);border-radius:50%;transform:rotateX(65deg);animation:spin 2s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-quantum": {
		name:     "Ring Quantum",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-quantum"><span></span><span></span></div>`,
		css: `.ring-quantum{position:relative;width:48px;height:48px}
.ring-quantum span{position:absolute;inset:0;border:3px solid transparent;border-top-color:var(--primary);border-bottom-color:var(--primary);border-radius:50%;animation:spin 1s linear infinite}
.ring-quantum span:nth-child(2){inset:8px;border-top-color:var(--secondary);border-bottom-color:var(--secondary);animation-direction:reverse;animation-duration:.8s}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-vortex": {
		name:     "Ring Vortex",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-vortex"><span></span><span></span><span></span></div>`,
		css: `.ring-vortex{position:relative;width:48px;height:48px}
.ring-vortex span{position:absolute;border:3px solid var(--primary);border-radius:50%;animation:spin 1.5s linear infinite}
.ring-vortex span:nth-child(1){inset:0}
.ring-vortex span:nth-child(2){inset:8px;animation-direction:reverse;animation-duration:1.2s}
.ring-vortex span:nth-child(3){inset:16px;animation-duration:.9s}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-segment": {
		name:     "Ring Segment",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-segment"></div>`,
		css: `.ring-segment{width:48px;height:48px;border:4px solid transparent;border-top-color:var(--primary);border-right-color:var(--primary);border-radius:50%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-arc": {
		name:     "Ring Arc",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-arc"></div>`,
		css: `.ring-arc{width:48px;height:48px;border:4px solid transparent;border-radius:50%;border-top-color:var(--primary);animation:spin 1s ease-in-out infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"ring-breathe": {
		name:     "Ring Breathe",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-breathe"></div>`,
		css: `.ring-breathe{width:48px;height:48px;border:3px solid var(--primary);border-radius:50%;animation:ring-breathe 2s ease-in-out infinite}
@keyframes ring-breathe{0%,100%{transform:scale(.8);opacity:.5}50%{transform:scale(1.1);opacity:1}}`,
	},
	"ring-svg": {
		name:     "Ring SVG",
		category: "Spinners & Rings",
		html:     `<div class="loader ring-svg"><svg viewBox="0 0 50 50"><circle cx="25" cy="25" r="20"></circle></svg></div>`,
		css: `.ring-svg{width:48px;height:48px}
.ring-svg svg{width:100%;height:100%;animation:spin 2s linear infinite}
.ring-svg circle{fill:none;stroke:var(--primary);stroke-width:4;stroke-linecap:round;stroke-dasharray:90 150;stroke-dashoffset:0;animation:ring-dash 1.5s ease-in-out infinite}
@keyframes ring-dash{0%{stroke-dasharray:1 150;stroke-dashoffset:0}50%{stroke-dasharray:90 150;stroke-dashoffset:-35}100%{stroke-dasharray:90 150;stroke-dashoffset:-124}}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},

	// =============================================================================
	// DOTS & BOUNCING (20)
	// =============================================================================

	"dots-bounce": {
		name:     "Dots Bounce",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-bounce"><span></span><span></span><span></span></div>`,
		css: `.dots-bounce{display:flex;gap:8px}
.dots-bounce span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-bounce .6s ease-in-out infinite alternate}
.dots-bounce span:nth-child(2){animation-delay:.2s}
.dots-bounce span:nth-child(3){animation-delay:.4s}
@keyframes dot-bounce{to{transform:translateY(-15px)}}`,
	},
	"dots-fade": {
		name:     "Dots Fade",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-fade"><span></span><span></span><span></span></div>`,
		css: `.dots-fade{display:flex;gap:8px}
.dots-fade span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-fade 1s ease-in-out infinite}
.dots-fade span:nth-child(2){animation-delay:.2s}
.dots-fade span:nth-child(3){animation-delay:.4s}
@keyframes dot-fade{0%,100%{opacity:.3}50%{opacity:1}}`,
	},
	"dots-scale": {
		name:     "Dots Scale",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-scale"><span></span><span></span><span></span></div>`,
		css: `.dots-scale{display:flex;gap:8px}
.dots-scale span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-scale .8s ease-in-out infinite}
.dots-scale span:nth-child(2){animation-delay:.15s}
.dots-scale span:nth-child(3){animation-delay:.3s}
@keyframes dot-scale{0%,100%{transform:scale(1)}50%{transform:scale(1.5)}}`,
	},
	"dots-wave": {
		name:     "Dots Wave",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-wave"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-wave{display:flex;gap:6px}
.dots-wave span{width:10px;height:10px;background:var(--primary);border-radius:50%;animation:dot-wave 1.2s ease-in-out infinite}
.dots-wave span:nth-child(2){animation-delay:.1s}
.dots-wave span:nth-child(3){animation-delay:.2s}
.dots-wave span:nth-child(4){animation-delay:.3s}
.dots-wave span:nth-child(5){animation-delay:.4s}
@keyframes dot-wave{0%,100%{transform:translateY(0)}50%{transform:translateY(-15px)}}`,
	},
	"dots-pulse": {
		name:     "Dots Pulse",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-pulse"><span></span><span></span><span></span></div>`,
		css: `.dots-pulse{display:flex;gap:8px}
.dots-pulse span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-pulse 1.4s ease-in-out infinite}
.dots-pulse span:nth-child(2){animation-delay:.2s}
.dots-pulse span:nth-child(3){animation-delay:.4s}
@keyframes dot-pulse{0%,100%{transform:scale(.6);opacity:.5}50%{transform:scale(1);opacity:1}}`,
	},
	"dots-typing": {
		name:     "Dots Typing",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-typing"><span></span><span></span><span></span></div>`,
		css: `.dots-typing{display:flex;gap:6px}
.dots-typing span{width:10px;height:10px;background:var(--primary);border-radius:50%;animation:dot-typing 1.4s infinite}
.dots-typing span:nth-child(2){animation-delay:.2s}
.dots-typing span:nth-child(3){animation-delay:.4s}
@keyframes dot-typing{0%,60%,100%{transform:translateY(0)}30%{transform:translateY(-10px)}}`,
	},
	"dots-flashing": {
		name:     "Dots Flashing",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-flashing"><span></span><span></span><span></span></div>`,
		css: `.dots-flashing{display:flex;gap:8px}
.dots-flashing span{width:12px;height:12px;background:var(--secondary);border-radius:50%;animation:dot-flash 1s infinite}
.dots-flashing span:nth-child(2){animation-delay:.25s}
.dots-flashing span:nth-child(3){animation-delay:.5s}
@keyframes dot-flash{0%,100%{background:var(--secondary)}50%{background:var(--primary)}}`,
	},
	"dots-elastic": {
		name:     "Dots Elastic",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-elastic"><span></span><span></span><span></span></div>`,
		css: `.dots-elastic{display:flex;gap:8px}
.dots-elastic span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-elastic .6s cubic-bezier(.5,0,.5,1) infinite alternate}
.dots-elastic span:nth-child(2){animation-delay:.1s}
.dots-elastic span:nth-child(3){animation-delay:.2s}
@keyframes dot-elastic{to{transform:translateY(-18px) scaleY(1.2)}}`,
	},
	"dots-collision": {
		name:     "Dots Collision",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-collision"><span></span><span></span></div>`,
		css: `.dots-collision{display:flex;width:60px;justify-content:space-between}
.dots-collision span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-collision 1s ease-in-out infinite}
.dots-collision span:nth-child(2){animation-direction:reverse}
@keyframes dot-collision{0%,100%{transform:translateX(0)}50%{transform:translateX(24px)}}`,
	},
	"dots-gather": {
		name:     "Dots Gather",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-gather"><span></span><span></span><span></span></div>`,
		css: `.dots-gather{position:relative;width:48px;height:48px}
.dots-gather span{position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-gather 1s ease-in-out infinite}
.dots-gather span:nth-child(1){top:0;left:0;animation-delay:0s}
.dots-gather span:nth-child(2){top:0;right:0;animation-delay:.2s}
.dots-gather span:nth-child(3){bottom:0;left:50%;margin-left:-6px;animation-delay:.4s}
@keyframes dot-gather{0%,100%{opacity:1}50%{opacity:.3;transform:translate(12px,12px)}}`,
	},
	"dots-spin": {
		name:     "Dots Spin",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-spin"><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-spin{position:relative;width:48px;height:48px;animation:spin 1.5s linear infinite}
.dots-spin span{position:absolute;width:10px;height:10px;background:var(--primary);border-radius:50%}
.dots-spin span:nth-child(1){top:0;left:50%;margin-left:-5px}
.dots-spin span:nth-child(2){bottom:0;left:50%;margin-left:-5px}
.dots-spin span:nth-child(3){top:50%;left:0;margin-top:-5px}
.dots-spin span:nth-child(4){top:50%;right:0;margin-top:-5px}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"dots-orbit": {
		name:     "Dots Orbit",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-orbit"><span></span><span></span></div>`,
		css: `.dots-orbit{position:relative;width:48px;height:48px}
.dots-orbit span{position:absolute;width:10px;height:10px;background:var(--primary);border-radius:50%;top:50%;left:50%;margin:-5px;animation:dots-orbit 1s linear infinite}
.dots-orbit span:nth-child(2){animation-delay:.5s;background:var(--secondary)}
@keyframes dots-orbit{to{transform:rotate(360deg) translateX(18px)}}`,
	},
	"dots-windmill": {
		name:     "Dots Windmill",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-windmill"><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-windmill{position:relative;width:48px;height:48px;animation:spin 1s linear infinite}
.dots-windmill span{position:absolute;width:8px;height:8px;background:var(--primary);border-radius:50%}
.dots-windmill span:nth-child(1){top:0;left:50%;margin-left:-4px}
.dots-windmill span:nth-child(2){top:50%;right:0;margin-top:-4px}
.dots-windmill span:nth-child(3){bottom:0;left:50%;margin-left:-4px}
.dots-windmill span:nth-child(4){top:50%;left:0;margin-top:-4px}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"dots-shuffle": {
		name:     "Dots Shuffle",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-shuffle"><span></span><span></span><span></span></div>`,
		css: `.dots-shuffle{display:flex;gap:8px}
.dots-shuffle span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-shuffle 1s ease-in-out infinite}
.dots-shuffle span:nth-child(1){animation-delay:0s}
.dots-shuffle span:nth-child(2){animation-delay:.1s}
.dots-shuffle span:nth-child(3){animation-delay:.2s}
@keyframes dot-shuffle{0%,100%{transform:translateX(0)}25%{transform:translateX(20px)}75%{transform:translateX(-20px)}}`,
	},
	"dots-chain": {
		name:     "Dots Chain",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-chain"><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-chain{display:flex;gap:4px}
.dots-chain span{width:8px;height:8px;background:var(--primary);border-radius:50%;animation:dot-chain 1.2s ease-in-out infinite}
.dots-chain span:nth-child(2){animation-delay:.15s}
.dots-chain span:nth-child(3){animation-delay:.3s}
.dots-chain span:nth-child(4){animation-delay:.45s}
@keyframes dot-chain{0%,100%{transform:scale(1);opacity:1}50%{transform:scale(.5);opacity:.5}}`,
	},
	"dots-falling": {
		name:     "Dots Falling",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-falling"><span></span><span></span><span></span></div>`,
		css: `.dots-falling{display:flex;gap:8px}
.dots-falling span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-falling .8s ease-in infinite}
.dots-falling span:nth-child(2){animation-delay:.15s}
.dots-falling span:nth-child(3){animation-delay:.3s}
@keyframes dot-falling{0%{transform:translateY(-20px);opacity:0}50%{opacity:1}100%{transform:translateY(0);opacity:1}}`,
	},
	"dots-triangle": {
		name:     "Dots Triangle",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-triangle"><span></span><span></span><span></span></div>`,
		css: `.dots-triangle{position:relative;width:48px;height:42px}
.dots-triangle span{position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-pulse 1s ease-in-out infinite}
.dots-triangle span:nth-child(1){top:0;left:50%;margin-left:-6px}
.dots-triangle span:nth-child(2){bottom:0;left:0;animation-delay:.3s}
.dots-triangle span:nth-child(3){bottom:0;right:0;animation-delay:.6s}`,
	},
	"dots-square": {
		name:     "Dots Square",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-square"><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-square{display:grid;grid-template-columns:1fr 1fr;gap:8px}
.dots-square span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-pulse 1.2s ease-in-out infinite}
.dots-square span:nth-child(2){animation-delay:.2s}
.dots-square span:nth-child(3){animation-delay:.6s}
.dots-square span:nth-child(4){animation-delay:.4s}`,
	},
	"dots-dna": {
		name:     "Dots DNA",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-dna"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-dna{display:flex;gap:4px;align-items:center}
.dots-dna span{width:8px;height:8px;background:var(--primary);border-radius:50%;animation:dna-wave 1s ease-in-out infinite}
.dots-dna span:nth-child(2){animation-delay:.1s}
.dots-dna span:nth-child(3){animation-delay:.2s}
.dots-dna span:nth-child(4){animation-delay:.3s}
.dots-dna span:nth-child(5){animation-delay:.4s}
@keyframes dna-wave{0%,100%{transform:translateY(-8px) scale(.8)}50%{transform:translateY(8px) scale(1.2)}}`,
	},
	"dots-matrix": {
		name:     "Dots Matrix",
		category: "Dots & Bouncing",
		html:     `<div class="loader dots-matrix"><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.dots-matrix{display:grid;grid-template-columns:repeat(3,12px);gap:4px}
.dots-matrix span{width:12px;height:12px;background:var(--primary);border-radius:50%;animation:dot-fade 1.2s ease-in-out infinite}
.dots-matrix span:nth-child(1){animation-delay:0s}
.dots-matrix span:nth-child(2){animation-delay:.1s}
.dots-matrix span:nth-child(3){animation-delay:.2s}
.dots-matrix span:nth-child(4){animation-delay:.1s}
.dots-matrix span:nth-child(5){animation-delay:.2s}
.dots-matrix span:nth-child(6){animation-delay:.3s}
.dots-matrix span:nth-child(7){animation-delay:.2s}
.dots-matrix span:nth-child(8){animation-delay:.3s}
.dots-matrix span:nth-child(9){animation-delay:.4s}`,
	},

	// =============================================================================
	// BARS & WAVES (13)
	// =============================================================================

	"bars-scale": {
		name:     "Bars Scale",
		category: "Bars & Waves",
		html:     `<div class="loader bars-scale"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-scale{display:flex;gap:4px;align-items:center;height:40px}
.bars-scale span{width:6px;height:20px;background:var(--primary);animation:bar-scale 1s ease-in-out infinite}
.bars-scale span:nth-child(2){animation-delay:.1s}
.bars-scale span:nth-child(3){animation-delay:.2s}
.bars-scale span:nth-child(4){animation-delay:.3s}
.bars-scale span:nth-child(5){animation-delay:.4s}
@keyframes bar-scale{0%,100%{height:20px}50%{height:40px}}`,
	},
	"bars-fade": {
		name:     "Bars Fade",
		category: "Bars & Waves",
		html:     `<div class="loader bars-fade"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-fade{display:flex;gap:4px;align-items:center}
.bars-fade span{width:6px;height:30px;background:var(--primary);animation:bar-fade 1s ease-in-out infinite}
.bars-fade span:nth-child(2){animation-delay:.1s}
.bars-fade span:nth-child(3){animation-delay:.2s}
.bars-fade span:nth-child(4){animation-delay:.3s}
.bars-fade span:nth-child(5){animation-delay:.4s}
@keyframes bar-fade{0%,100%{opacity:.3}50%{opacity:1}}`,
	},
	"bars-bounce": {
		name:     "Bars Bounce",
		category: "Bars & Waves",
		html:     `<div class="loader bars-bounce"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-bounce{display:flex;gap:4px;align-items:flex-end;height:40px}
.bars-bounce span{width:6px;height:30px;background:var(--primary);animation:bar-bounce 1s ease-in-out infinite}
.bars-bounce span:nth-child(2){animation-delay:.1s}
.bars-bounce span:nth-child(3){animation-delay:.2s}
.bars-bounce span:nth-child(4){animation-delay:.3s}
.bars-bounce span:nth-child(5){animation-delay:.4s}
@keyframes bar-bounce{0%,100%{transform:translateY(0)}50%{transform:translateY(-10px)}}`,
	},
	"bars-wave": {
		name:     "Bars Wave",
		category: "Bars & Waves",
		html:     `<div class="loader bars-wave"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-wave{display:flex;gap:4px;align-items:center;height:40px}
.bars-wave span{width:6px;height:10px;background:var(--primary);animation:bar-wave 1.2s ease-in-out infinite}
.bars-wave span:nth-child(2){animation-delay:.1s}
.bars-wave span:nth-child(3){animation-delay:.2s}
.bars-wave span:nth-child(4){animation-delay:.3s}
.bars-wave span:nth-child(5){animation-delay:.4s}
@keyframes bar-wave{0%,100%{height:10px}50%{height:40px}}`,
	},
	"bars-audio": {
		name:     "Bars Audio",
		category: "Bars & Waves",
		html:     `<div class="loader bars-audio"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-audio{display:flex;gap:3px;align-items:flex-end;height:40px}
.bars-audio span{width:5px;background:var(--primary);animation:bar-audio 1s ease-in-out infinite}
.bars-audio span:nth-child(1){animation-delay:0s}
.bars-audio span:nth-child(2){animation-delay:.2s}
.bars-audio span:nth-child(3){animation-delay:.4s}
.bars-audio span:nth-child(4){animation-delay:.6s}
.bars-audio span:nth-child(5){animation-delay:.8s}
@keyframes bar-audio{0%,100%{height:10px}25%{height:35px}50%{height:20px}75%{height:40px}}`,
	},
	"bars-equalizer": {
		name:     "Bars Equalizer",
		category: "Bars & Waves",
		html:     `<div class="loader bars-equalizer"><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-equalizer{display:flex;gap:4px;align-items:flex-end;height:40px}
.bars-equalizer span{width:8px;background:var(--primary);animation:bar-eq .8s ease-in-out infinite}
.bars-equalizer span:nth-child(1){animation-delay:0s}
.bars-equalizer span:nth-child(2){animation-delay:.15s}
.bars-equalizer span:nth-child(3){animation-delay:.3s}
.bars-equalizer span:nth-child(4){animation-delay:.45s}
@keyframes bar-eq{0%,100%{height:15px}50%{height:40px}}`,
	},
	"bars-progress": {
		name:     "Bars Progress",
		category: "Bars & Waves",
		html:     `<div class="loader bars-progress"><span></span></div>`,
		css: `.bars-progress{width:60px;height:8px;background:var(--secondary);border-radius:4px;overflow:hidden}
.bars-progress span{display:block;height:100%;background:var(--primary);animation:bar-progress 1.5s ease-in-out infinite}
@keyframes bar-progress{0%{width:0}50%{width:100%}100%{width:0}}`,
	},
	"bars-loading": {
		name:     "Bars Loading",
		category: "Bars & Waves",
		html:     `<div class="loader bars-loading"><span></span><span></span><span></span></div>`,
		css: `.bars-loading{display:flex;gap:3px;align-items:center}
.bars-loading span{width:4px;height:20px;background:var(--primary);border-radius:2px;animation:bar-loading .8s ease-in-out infinite}
.bars-loading span:nth-child(2){animation-delay:.15s}
.bars-loading span:nth-child(3){animation-delay:.3s}
@keyframes bar-loading{0%,100%{transform:scaleY(1)}50%{transform:scaleY(2)}}`,
	},
	"bars-stretch": {
		name:     "Bars Stretch",
		category: "Bars & Waves",
		html:     `<div class="loader bars-stretch"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-stretch{display:flex;gap:3px;align-items:center;height:40px}
.bars-stretch span{width:5px;height:40px;background:var(--primary);animation:bar-stretch 1.2s ease-in-out infinite}
.bars-stretch span:nth-child(2){animation-delay:.1s}
.bars-stretch span:nth-child(3){animation-delay:.2s}
.bars-stretch span:nth-child(4){animation-delay:.3s}
.bars-stretch span:nth-child(5){animation-delay:.4s}
@keyframes bar-stretch{0%,100%{transform:scaleY(1)}50%{transform:scaleY(.5)}}`,
	},
	"bars-music": {
		name:     "Bars Music",
		category: "Bars & Waves",
		html:     `<div class="loader bars-music"><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-music{display:flex;gap:4px;align-items:flex-end;height:40px}
.bars-music span{width:6px;background:var(--primary);border-radius:3px 3px 0 0;animation:bar-music .6s ease-in-out infinite alternate}
.bars-music span:nth-child(1){animation-duration:.5s}
.bars-music span:nth-child(2){animation-duration:.7s}
.bars-music span:nth-child(3){animation-duration:.6s}
.bars-music span:nth-child(4){animation-duration:.8s}
@keyframes bar-music{from{height:10px}to{height:40px}}`,
	},
	"bars-signal": {
		name:     "Bars Signal",
		category: "Bars & Waves",
		html:     `<div class="loader bars-signal"><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-signal{display:flex;gap:4px;align-items:flex-end;height:40px}
.bars-signal span{width:8px;background:var(--primary);border-radius:2px;animation:bar-signal 1.2s ease-in-out infinite}
.bars-signal span:nth-child(1){height:10px;animation-delay:0s}
.bars-signal span:nth-child(2){height:20px;animation-delay:.15s}
.bars-signal span:nth-child(3){height:30px;animation-delay:.3s}
.bars-signal span:nth-child(4){height:40px;animation-delay:.45s}
@keyframes bar-signal{0%,100%{opacity:1}50%{opacity:.3}}`,
	},
	"bars-wifi": {
		name:     "Bars Wifi",
		category: "Bars & Waves",
		html:     `<div class="loader bars-wifi"><span></span><span></span><span></span></div>`,
		css: `.bars-wifi{position:relative;width:40px;height:40px}
.bars-wifi span{position:absolute;bottom:0;left:50%;transform:translateX(-50%);border:3px solid transparent;border-bottom-color:var(--primary);border-radius:0 0 50% 50%;animation:wifi-pulse 1.5s ease-out infinite}
.bars-wifi span:nth-child(1){width:12px;height:6px;animation-delay:0s}
.bars-wifi span:nth-child(2){width:24px;height:12px;animation-delay:.3s}
.bars-wifi span:nth-child(3){width:36px;height:18px;animation-delay:.6s}
@keyframes wifi-pulse{0%{opacity:0}50%{opacity:1}100%{opacity:0}}`,
	},
	"bars-spectrum": {
		name:     "Bars Spectrum",
		category: "Bars & Waves",
		html:     `<div class="loader bars-spectrum"><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.bars-spectrum{display:flex;gap:2px;align-items:flex-end;height:40px}
.bars-spectrum span{width:4px;background:var(--primary);animation:bar-spectrum .8s ease-in-out infinite}
.bars-spectrum span:nth-child(1){animation-delay:0s}
.bars-spectrum span:nth-child(2){animation-delay:.1s}
.bars-spectrum span:nth-child(3){animation-delay:.2s}
.bars-spectrum span:nth-child(4){animation-delay:.3s}
.bars-spectrum span:nth-child(5){animation-delay:.4s}
.bars-spectrum span:nth-child(6){animation-delay:.5s}
.bars-spectrum span:nth-child(7){animation-delay:.6s}
@keyframes bar-spectrum{0%,100%{height:8px}50%{height:40px}}`,
	},

	// =============================================================================
	// SHAPES & MORPHING (17)
	// =============================================================================

	"square-spin": {
		name:     "Square Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader square-spin"></div>`,
		css: `.square-spin{width:40px;height:40px;background:var(--primary);animation:sq-spin 1.2s ease-in-out infinite}
@keyframes sq-spin{25%{transform:perspective(100px) rotateX(180deg)}50%{transform:perspective(100px) rotateX(180deg) rotateY(180deg)}75%{transform:perspective(100px) rotateY(180deg)}100%{transform:perspective(100px) rotateX(360deg) rotateY(360deg)}}`,
	},
	"square-flip": {
		name:     "Square Flip",
		category: "Shapes & Morphing",
		html:     `<div class="loader square-flip"></div>`,
		css: `.square-flip{width:40px;height:40px;background:var(--primary);animation:square-flip 1s ease-in-out infinite}
@keyframes square-flip{0%{transform:perspective(120px) rotateX(0) rotateY(0)}50%{transform:perspective(120px) rotateX(180deg) rotateY(0)}100%{transform:perspective(120px) rotateX(180deg) rotateY(180deg)}}`,
	},
	"square-morph": {
		name:     "Square Morph",
		category: "Shapes & Morphing",
		html:     `<div class="loader square-morph"></div>`,
		css: `.square-morph{width:40px;height:40px;background:var(--primary);animation:square-morph 1.2s ease-in-out infinite}
@keyframes square-morph{0%,100%{border-radius:0}50%{border-radius:50%}}`,
	},
	"square-fold": {
		name:     "Square Fold",
		category: "Shapes & Morphing",
		html:     `<div class="loader square-fold"><span></span><span></span><span></span><span></span></div>`,
		css: `.square-fold{position:relative;width:40px;height:40px;transform:rotate(45deg)}
.square-fold span{position:absolute;width:20px;height:20px;background:var(--primary);animation:square-fold 2s ease-in-out infinite}
.square-fold span:nth-child(1){top:0;left:0;animation-delay:0s}
.square-fold span:nth-child(2){top:0;right:0;animation-delay:.5s}
.square-fold span:nth-child(3){bottom:0;right:0;animation-delay:1s}
.square-fold span:nth-child(4){bottom:0;left:0;animation-delay:1.5s}
@keyframes square-fold{0%,100%{transform:scale(1)}50%{transform:scale(.5)}}`,
	},
	"cube-spin": {
		name:     "Cube Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader cube-spin"></div>`,
		css: `.cube-spin{width:40px;height:40px;background:var(--primary);animation:cube-spin 1.2s ease-in-out infinite}
@keyframes cube-spin{0%{transform:perspective(150px) rotateX(0) rotateY(0)}50%{transform:perspective(150px) rotateX(180deg) rotateY(0)}100%{transform:perspective(150px) rotateX(180deg) rotateY(180deg)}}`,
	},
	"cube-grid": {
		name:     "Cube Grid",
		category: "Shapes & Morphing",
		html:     `<div class="loader cube-grid"><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.cube-grid{display:grid;grid-template-columns:repeat(3,12px);gap:4px}
.cube-grid span{width:12px;height:12px;background:var(--primary);animation:cube-grid 1.3s ease-in-out infinite}
.cube-grid span:nth-child(1){animation-delay:.2s}
.cube-grid span:nth-child(2){animation-delay:.3s}
.cube-grid span:nth-child(3){animation-delay:.4s}
.cube-grid span:nth-child(4){animation-delay:.1s}
.cube-grid span:nth-child(5){animation-delay:.2s}
.cube-grid span:nth-child(6){animation-delay:.3s}
.cube-grid span:nth-child(7){animation-delay:0s}
.cube-grid span:nth-child(8){animation-delay:.1s}
.cube-grid span:nth-child(9){animation-delay:.2s}
@keyframes cube-grid{0%,100%{transform:scale(1)}50%{transform:scale(.5)}}`,
	},
	"cube-move": {
		name:     "Cube Move",
		category: "Shapes & Morphing",
		html:     `<div class="loader cube-move"><span></span><span></span></div>`,
		css: `.cube-move{position:relative;width:60px;height:30px}
.cube-move span{position:absolute;width:20px;height:20px;background:var(--primary);animation:cube-move 1.2s ease-in-out infinite}
.cube-move span:nth-child(2){animation-delay:.6s;background:var(--secondary)}
@keyframes cube-move{0%{left:0}50%{left:40px}100%{left:0}}`,
	},
	"triangle-spin": {
		name:     "Triangle Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader triangle-spin"></div>`,
		css: `.triangle-spin{width:0;height:0;border-left:25px solid transparent;border-right:25px solid transparent;border-bottom:43px solid var(--primary);animation:spin 1.2s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"hexagon-spin": {
		name:     "Hexagon Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader hexagon-spin"></div>`,
		css: `.hexagon-spin{width:40px;height:46px;background:var(--primary);clip-path:polygon(50% 0%,100% 25%,100% 75%,50% 100%,0% 75%,0% 25%);animation:spin 1.5s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"hexagon-pulse": {
		name:     "Hexagon Pulse",
		category: "Shapes & Morphing",
		html:     `<div class="loader hexagon-pulse"></div>`,
		css: `.hexagon-pulse{width:40px;height:46px;background:var(--primary);clip-path:polygon(50% 0%,100% 25%,100% 75%,50% 100%,0% 75%,0% 25%);animation:hex-pulse 1s ease-in-out infinite}
@keyframes hex-pulse{0%,100%{transform:scale(1);opacity:1}50%{transform:scale(.8);opacity:.5}}`,
	},
	"diamond-spin": {
		name:     "Diamond Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader diamond-spin"></div>`,
		css: `.diamond-spin{width:30px;height:30px;background:var(--primary);transform:rotate(45deg);animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"star-spin": {
		name:     "Star Spin",
		category: "Shapes & Morphing",
		html:     `<div class="loader star-spin"></div>`,
		css: `.star-spin{width:40px;height:40px;background:var(--primary);clip-path:polygon(50% 0%,61% 35%,98% 35%,68% 57%,79% 91%,50% 70%,21% 91%,32% 57%,2% 35%,39% 35%);animation:spin 1.5s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"blob-morph": {
		name:     "Blob Morph",
		category: "Shapes & Morphing",
		html:     `<div class="loader blob-morph"></div>`,
		css: `.blob-morph{width:48px;height:48px;background:var(--primary);animation:blob-morph 2s ease-in-out infinite}
@keyframes blob-morph{0%,100%{border-radius:60% 40% 30% 70%/60% 30% 70% 40%}25%{border-radius:30% 60% 70% 40%/50% 60% 30% 60%}50%{border-radius:50% 60% 30% 60%/30% 60% 70% 40%}75%{border-radius:60% 40% 60% 30%/70% 30% 50% 60%}}`,
	},
	"blob-bounce": {
		name:     "Blob Bounce",
		category: "Shapes & Morphing",
		html:     `<div class="loader blob-bounce"></div>`,
		css: `.blob-bounce{width:48px;height:48px;background:var(--primary);border-radius:50%;animation:blob-bounce .6s ease-in-out infinite alternate}
@keyframes blob-bounce{0%{transform:translateY(0) scaleX(1) scaleY(1)}100%{transform:translateY(-20px) scaleX(.9) scaleY(1.1)}}`,
	},
	"heart-beat": {
		name:     "Heart Beat",
		category: "Shapes & Morphing",
		html:     `<div class="loader heart-beat"></div>`,
		css: `.heart-beat{width:40px;height:36px;background:var(--primary);position:relative;transform:rotate(-45deg);animation:heart-beat 1s ease-in-out infinite}
.heart-beat::before,.heart-beat::after{content:'';position:absolute;width:40px;height:40px;background:var(--primary);border-radius:50%}
.heart-beat::before{top:-20px;left:0}
.heart-beat::after{left:20px;top:0}
@keyframes heart-beat{0%,100%{transform:rotate(-45deg) scale(1)}50%{transform:rotate(-45deg) scale(1.15)}}`,
	},
	"pacman": {
		name:     "Pacman",
		category: "Shapes & Morphing",
		html:     `<div class="loader pacman"><span></span></div>`,
		css: `.pacman{position:relative;width:48px;height:48px}
.pacman::before,.pacman::after{content:'';position:absolute;width:48px;height:24px;background:var(--primary)}
.pacman::before{border-radius:48px 48px 0 0;animation:pacman-top .5s ease-in-out infinite}
.pacman::after{top:24px;border-radius:0 0 48px 48px;animation:pacman-bottom .5s ease-in-out infinite}
.pacman span{position:absolute;width:8px;height:8px;background:var(--secondary);border-radius:50%;top:20px;left:60px;animation:pacman-dot .5s linear infinite}
@keyframes pacman-top{0%,100%{transform:rotate(0)}50%{transform:rotate(-35deg)}}
@keyframes pacman-bottom{0%,100%{transform:rotate(0)}50%{transform:rotate(35deg)}}
@keyframes pacman-dot{to{transform:translateX(-60px);opacity:0}}`,
	},
	"infinity": {
		name:     "Infinity",
		category: "Shapes & Morphing",
		html:     `<div class="loader infinity"><span></span></div>`,
		css: `.infinity{position:relative;width:80px;height:40px}
.infinity::before,.infinity::after{content:'';position:absolute;width:32px;height:32px;border:4px solid var(--secondary);border-radius:50%;top:0}
.infinity::before{left:0}
.infinity::after{right:0}
.infinity span{position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;animation:infinity 2s linear infinite}
@keyframes infinity{0%{left:4px;top:10px}25%{left:calc(50% - 6px);top:0}50%{left:calc(100% - 16px);top:10px}75%{left:calc(50% - 6px);top:20px}100%{left:4px;top:10px}}`,
	},
	// =============================================================================
	// PULSE & GLOW (13)
	// =============================================================================

	"pulse-circle": {
		name:     "Pulse Circle",
		category: "Pulse & Glow",
		html:     `<div class="loader pulse-circle"></div>`,
		css: `.pulse-circle{width:48px;height:48px;background:var(--primary);border-radius:50%;animation:pulse-circle 1.2s ease-in-out infinite}
@keyframes pulse-circle{0%,100%{transform:scale(1);opacity:1}50%{transform:scale(.8);opacity:.5}}`,
	},
	"pulse-ring": {
		name:     "Pulse Ring",
		category: "Pulse & Glow",
		html:     `<div class="loader pulse-ring"><span></span><span></span><span></span></div>`,
		css: `.pulse-ring{position:relative;width:48px;height:48px}
.pulse-ring span{position:absolute;inset:0;border:3px solid var(--primary);border-radius:50%;animation:pulse-ring 1.5s ease-out infinite}
.pulse-ring span:nth-child(2){animation-delay:.3s}
.pulse-ring span:nth-child(3){animation-delay:.6s}
@keyframes pulse-ring{0%{transform:scale(.5);opacity:1}100%{transform:scale(1.3);opacity:0}}`,
	},
	"pulse-dot": {
		name:     "Pulse Dot",
		category: "Pulse & Glow",
		html:     `<div class="loader pulse-dot"></div>`,
		css: `.pulse-dot{width:20px;height:20px;background:var(--primary);border-radius:50%;animation:pulse-dot 1s ease-in-out infinite}
@keyframes pulse-dot{0%,100%{transform:scale(1)}50%{transform:scale(1.5)}}`,
	},
	"glow-circle": {
		name:     "Glow Circle",
		category: "Pulse & Glow",
		html:     `<div class="loader glow-circle"></div>`,
		css: `.glow-circle{width:48px;height:48px;background:var(--primary);border-radius:50%;animation:glow-circle 1.5s ease-in-out infinite}
@keyframes glow-circle{0%,100%{box-shadow:0 0 5px var(--primary)}50%{box-shadow:0 0 25px var(--primary),0 0 50px var(--primary)}}`,
	},
	"glow-ring": {
		name:     "Glow Ring",
		category: "Pulse & Glow",
		html:     `<div class="loader glow-ring"></div>`,
		css: `.glow-ring{width:48px;height:48px;border:4px solid var(--primary);border-radius:50%;animation:glow-ring 1.5s ease-in-out infinite}
@keyframes glow-ring{0%,100%{box-shadow:0 0 5px var(--primary),inset 0 0 5px var(--primary)}50%{box-shadow:0 0 20px var(--primary),inset 0 0 15px var(--primary)}}`,
	},
	"neon-circle": {
		name:     "Neon Circle",
		category: "Pulse & Glow",
		html:     `<div class="loader neon-circle"></div>`,
		css: `.neon-circle{width:48px;height:48px;border:3px solid var(--primary);border-radius:50%;animation:neon-pulse 1s ease-in-out infinite,spin 2s linear infinite}
@keyframes neon-pulse{0%,100%{box-shadow:0 0 5px var(--primary),0 0 10px var(--primary),0 0 15px var(--primary)}50%{box-shadow:0 0 10px var(--primary),0 0 20px var(--primary),0 0 30px var(--primary)}}`,
	},
	"neon-square": {
		name:     "Neon Square",
		category: "Pulse & Glow",
		html:     `<div class="loader neon-square"></div>`,
		css: `.neon-square{width:40px;height:40px;border:3px solid var(--primary);animation:neon-pulse 1s ease-in-out infinite}`,
	},
	"radar": {
		name:     "Radar",
		category: "Pulse & Glow",
		html:     `<div class="loader radar"><span></span></div>`,
		css: `.radar{position:relative;width:48px;height:48px;border:2px solid var(--secondary);border-radius:50%}
.radar::before{content:'';position:absolute;width:4px;height:4px;background:var(--primary);border-radius:50%;top:50%;left:50%;transform:translate(-50%,-50%)}
.radar span{position:absolute;inset:0;border-radius:50%;background:conic-gradient(transparent 0deg,var(--primary) 90deg,transparent 90deg);animation:spin 1.5s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"sonar": {
		name:     "Sonar",
		category: "Pulse & Glow",
		html:     `<div class="loader sonar"><span></span><span></span></div>`,
		css: `.sonar{position:relative;width:48px;height:48px}
.sonar::before{content:'';position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;top:50%;left:50%;transform:translate(-50%,-50%)}
.sonar span{position:absolute;inset:0;border:2px solid var(--primary);border-radius:50%;animation:sonar 2s ease-out infinite}
.sonar span:nth-child(2){animation-delay:1s}
@keyframes sonar{0%{transform:scale(.5);opacity:1}100%{transform:scale(1.5);opacity:0}}`,
	},
	"beacon": {
		name:     "Beacon",
		category: "Pulse & Glow",
		html:     `<div class="loader beacon"></div>`,
		css: `.beacon{position:relative;width:20px;height:20px;background:var(--primary);border-radius:50%}
.beacon::before,.beacon::after{content:'';position:absolute;inset:-10px;border:3px solid var(--primary);border-radius:50%;animation:beacon 1.5s ease-out infinite}
.beacon::after{animation-delay:.5s}
@keyframes beacon{0%{transform:scale(.8);opacity:1}100%{transform:scale(1.8);opacity:0}}`,
	},
	"ripple": {
		name:     "Ripple",
		category: "Pulse & Glow",
		html:     `<div class="loader ripple"><span></span><span></span><span></span></div>`,
		css: `.ripple{position:relative;width:48px;height:48px}
.ripple span{position:absolute;inset:0;border:3px solid var(--primary);border-radius:50%;animation:ripple 1.5s linear infinite}
.ripple span:nth-child(2){animation-delay:.5s}
.ripple span:nth-child(3){animation-delay:1s}
@keyframes ripple{0%{transform:scale(0);opacity:1}100%{transform:scale(1);opacity:0}}`,
	},
	"breathing": {
		name:     "Breathing",
		category: "Pulse & Glow",
		html:     `<div class="loader breathing"></div>`,
		css: `.breathing{width:48px;height:48px;background:var(--primary);border-radius:50%;animation:breathing 2s ease-in-out infinite}
@keyframes breathing{0%,100%{transform:scale(.9);opacity:.7}50%{transform:scale(1.1);opacity:1}}`,
	},
	"throb": {
		name:     "Throb",
		category: "Pulse & Glow",
		html:     `<div class="loader throb"></div>`,
		css: `.throb{width:48px;height:48px;background:var(--primary);border-radius:50%;animation:throb 1s ease-in-out infinite}
@keyframes throb{0%,100%{transform:scale(1)}25%{transform:scale(1.1)}50%{transform:scale(1)}75%{transform:scale(1.1)}}`,
	},

	// =============================================================================
	// PROGRESS & SPECIAL (11)
	// =============================================================================

	"progress-bar": {
		name:     "Progress Bar",
		category: "Progress & Special",
		html:     `<div class="loader progress-bar"><span></span></div>`,
		css: `.progress-bar{width:80px;height:6px;background:var(--secondary);border-radius:3px;overflow:hidden}
.progress-bar span{display:block;height:100%;background:var(--primary);animation:progress-bar 2s ease-in-out infinite}
@keyframes progress-bar{0%{width:0;margin-left:0}50%{width:100%;margin-left:0}100%{width:0;margin-left:100%}}`,
	},
	"progress-circle": {
		name:     "Progress Circle",
		category: "Progress & Special",
		html:     `<div class="loader progress-circle"><svg viewBox="0 0 50 50"><circle cx="25" cy="25" r="20"/></svg></div>`,
		css: `.progress-circle{width:48px;height:48px}
.progress-circle svg{width:100%;height:100%;transform:rotate(-90deg)}
.progress-circle circle{fill:none;stroke:var(--primary);stroke-width:4;stroke-linecap:round;stroke-dasharray:125.6;animation:progress-circle 2s ease-in-out infinite}
@keyframes progress-circle{0%{stroke-dashoffset:125.6}50%{stroke-dashoffset:0}100%{stroke-dashoffset:-125.6}}`,
	},
	"hourglass": {
		name:     "Hourglass",
		category: "Progress & Special",
		html:     `<div class="loader hourglass"></div>`,
		css: `.hourglass{width:40px;height:40px;border:4px solid var(--primary);border-radius:50% 0;animation:hourglass 1.5s ease-in-out infinite}
@keyframes hourglass{0%{transform:rotate(0)}50%{transform:rotate(180deg);border-radius:0 50%}100%{transform:rotate(360deg)}}`,
	},
	"clock": {
		name:     "Clock",
		category: "Progress & Special",
		html:     `<div class="loader clock"><span></span><span></span></div>`,
		css: `.clock{position:relative;width:48px;height:48px;border:3px solid var(--primary);border-radius:50%}
.clock span{position:absolute;background:var(--primary);transform-origin:bottom center;left:50%;bottom:50%}
.clock span:nth-child(1){width:3px;height:14px;margin-left:-1.5px;animation:spin 2s linear infinite}
.clock span:nth-child(2){width:2px;height:18px;margin-left:-1px;animation:spin 8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
	"fill-circle": {
		name:     "Fill Circle",
		category: "Progress & Special",
		html:     `<div class="loader fill-circle"></div>`,
		css: `.fill-circle{width:48px;height:48px;border:3px solid var(--secondary);border-radius:50%;position:relative;overflow:hidden}
.fill-circle::before{content:'';position:absolute;bottom:0;left:0;right:0;background:var(--primary);animation:fill-circle 2s ease-in-out infinite}
@keyframes fill-circle{0%{height:0}50%{height:100%}100%{height:0}}`,
	},
	"bouncing-ball": {
		name:     "Bouncing Ball",
		category: "Progress & Special",
		html:     `<div class="loader bouncing-ball"><span></span></div>`,
		css: `.bouncing-ball{position:relative;width:60px;height:50px}
.bouncing-ball::before{content:'';position:absolute;bottom:0;left:50%;width:40px;height:4px;margin-left:-20px;background:var(--secondary);border-radius:50%;animation:ball-shadow .6s ease-in-out infinite}
.bouncing-ball span{position:absolute;width:20px;height:20px;background:var(--primary);border-radius:50%;left:50%;margin-left:-10px;animation:ball-bounce .6s ease-in-out infinite}
@keyframes ball-bounce{0%,100%{bottom:30px;transform:scaleY(1)}50%{bottom:4px;transform:scaleY(.8)}}
@keyframes ball-shadow{0%,100%{transform:scale(.7);opacity:.5}50%{transform:scale(1);opacity:.3}}`,
	},
	"pendulum": {
		name:     "Pendulum",
		category: "Progress & Special",
		html:     `<div class="loader pendulum"><span></span></div>`,
		css: `.pendulum{position:relative;width:60px;height:50px}
.pendulum::before{content:'';position:absolute;top:0;left:50%;width:2px;height:35px;background:var(--secondary);transform-origin:top center;animation:pendulum 1s ease-in-out infinite}
.pendulum span{position:absolute;width:16px;height:16px;background:var(--primary);border-radius:50%;top:28px;left:50%;margin-left:-8px;animation:pendulum 1s ease-in-out infinite}
@keyframes pendulum{0%,100%{transform:rotate(-30deg)}50%{transform:rotate(30deg)}}`,
	},
	"orbit": {
		name:     "Orbit",
		category: "Progress & Special",
		html:     `<div class="loader orbit"><span></span><span></span></div>`,
		css: `.orbit{position:relative;width:48px;height:48px}
.orbit span{position:absolute;width:12px;height:12px;background:var(--primary);border-radius:50%;animation:orbit-anim 1.2s linear infinite}
.orbit span:nth-child(1){top:0;left:50%;margin-left:-6px;animation-delay:0s}
.orbit span:nth-child(2){background:var(--secondary);bottom:0;left:50%;margin-left:-6px;animation-delay:.6s}
@keyframes orbit-anim{0%{transform:rotate(0deg) translateY(18px)}100%{transform:rotate(360deg) translateY(18px)}}`,
	},
	"meteor": {
		name:     "Meteor",
		category: "Progress & Special",
		html:     `<div class="loader meteor"></div>`,
		css: `.meteor{width:10px;height:10px;background:var(--primary);border-radius:50%;position:relative;animation:meteor 1.5s ease-in-out infinite}
.meteor::before{content:'';position:absolute;width:40px;height:4px;background:linear-gradient(to left,var(--primary),transparent);right:10px;top:3px;border-radius:2px}
@keyframes meteor{0%{transform:translateX(-30px) translateY(30px);opacity:0}20%{opacity:1}80%{opacity:1}100%{transform:translateX(30px) translateY(-30px);opacity:0}}`,
	},
	"galaxy": {
		name:     "Galaxy",
		category: "Progress & Special",
		html:     `<div class="loader galaxy"><span></span><span></span><span></span></div>`,
		css: `.galaxy{position:relative;width:48px;height:48px;animation:spin 3s linear infinite}
.galaxy span{position:absolute;width:8px;height:8px;background:var(--primary);border-radius:50%;top:50%;left:50%;transform-origin:0 0}
.galaxy span:nth-child(1){animation:galaxy-orbit 1s linear infinite;transform:translateX(16px)}
.galaxy span:nth-child(2){animation:galaxy-orbit 1.5s linear infinite;transform:translateX(20px);width:6px;height:6px}
.galaxy span:nth-child(3){animation:galaxy-orbit 2s linear infinite;transform:translateX(24px);width:4px;height:4px;background:var(--secondary)}
@keyframes spin{to{transform:rotate(360deg)}}
@keyframes galaxy-orbit{to{transform:rotate(360deg) translateX(20px)}}`,
	},
	"dna-helix": {
		name:     "DNA Helix",
		category: "Progress & Special",
		html:     `<div class="loader dna-helix"><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.dna-helix{display:flex;gap:3px;align-items:center;height:40px}
.dna-helix span{width:6px;height:6px;background:var(--primary);border-radius:50%;animation:dna-helix 1s ease-in-out infinite}
.dna-helix span:nth-child(odd){animation-name:dna-helix-up}
.dna-helix span:nth-child(even){animation-name:dna-helix-down;background:var(--secondary)}
.dna-helix span:nth-child(1),.dna-helix span:nth-child(2){animation-delay:0s}
.dna-helix span:nth-child(3),.dna-helix span:nth-child(4){animation-delay:.1s}
.dna-helix span:nth-child(5),.dna-helix span:nth-child(6){animation-delay:.2s}
.dna-helix span:nth-child(7),.dna-helix span:nth-child(8){animation-delay:.3s}
.dna-helix span:nth-child(9),.dna-helix span:nth-child(10){animation-delay:.4s}
@keyframes dna-helix-up{0%,100%{transform:translateY(-12px)}50%{transform:translateY(12px)}}
@keyframes dna-helix-down{0%,100%{transform:translateY(12px)}50%{transform:translateY(-12px)}}`,
	},

	// =============================================================================
	// BRAND INSPIRED (9)
	// =============================================================================

	"google-dots": {
		name:     "Google Dots",
		category: "Brand Inspired",
		html:     `<div class="loader google-dots"><span></span><span></span><span></span><span></span></div>`,
		css: `.google-dots{display:flex;gap:8px}
.google-dots span{width:12px;height:12px;border-radius:50%;animation:g-dots 1.4s ease-in-out infinite both}
.google-dots span:nth-child(1){background:#4285f4;animation-delay:-.32s}
.google-dots span:nth-child(2){background:#ea4335;animation-delay:-.16s}
.google-dots span:nth-child(3){background:#fbbc04}
.google-dots span:nth-child(4){background:#34a853;animation-delay:.16s}
@keyframes g-dots{0%,80%,100%{transform:scale(0)}40%{transform:scale(1)}}`,
	},
	"spotify-bars": {
		name:     "Spotify Bars",
		category: "Brand Inspired",
		html:     `<div class="loader spotify-bars"><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.spotify-bars{display:flex;gap:4px;height:40px;align-items:flex-end}
.spotify-bars span{width:6px;background:#1db954;border-radius:2px;animation:s-bars 1.2s ease-in-out infinite}
.spotify-bars span:nth-child(1){animation-delay:0s}
.spotify-bars span:nth-child(2){animation-delay:.1s}
.spotify-bars span:nth-child(3){animation-delay:.2s}
.spotify-bars span:nth-child(4){animation-delay:.3s}
.spotify-bars span:nth-child(5){animation-delay:.4s}
@keyframes s-bars{0%,100%{height:10px}50%{height:40px}}`,
	},
	"microsoft-squares": {
		name:     "Microsoft Squares",
		category: "Brand Inspired",
		html:     `<div class="loader microsoft-squares"><span></span><span></span><span></span><span></span></div>`,
		css: `.microsoft-squares{width:48px;height:48px;display:grid;grid-template-columns:1fr 1fr;gap:4px}
.microsoft-squares span{animation:ms-sq 2s ease-in-out infinite}
.microsoft-squares span:nth-child(1){background:#f25022}
.microsoft-squares span:nth-child(2){background:#7fba00;animation-delay:.2s}
.microsoft-squares span:nth-child(3){background:#00a4ef;animation-delay:.4s}
.microsoft-squares span:nth-child(4){background:#ffb900;animation-delay:.6s}
@keyframes ms-sq{0%,100%{transform:scale(1)}50%{transform:scale(.8)}}`,
	},
	"apple-spinner": {
		name:     "Apple Spinner",
		category: "Brand Inspired",
		html:     `<div class="loader apple-spinner"><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span><span></span></div>`,
		css: `.apple-spinner{position:relative;width:40px;height:40px}
.apple-spinner span{position:absolute;width:3px;height:10px;background:var(--primary);border-radius:2px;left:50%;top:4px;margin-left:-1.5px;transform-origin:center 16px;animation:apple-fade 1s linear infinite}
.apple-spinner span:nth-child(1){transform:rotate(0deg);animation-delay:0s}
.apple-spinner span:nth-child(2){transform:rotate(30deg);animation-delay:.083s}
.apple-spinner span:nth-child(3){transform:rotate(60deg);animation-delay:.166s}
.apple-spinner span:nth-child(4){transform:rotate(90deg);animation-delay:.25s}
.apple-spinner span:nth-child(5){transform:rotate(120deg);animation-delay:.333s}
.apple-spinner span:nth-child(6){transform:rotate(150deg);animation-delay:.416s}
.apple-spinner span:nth-child(7){transform:rotate(180deg);animation-delay:.5s}
.apple-spinner span:nth-child(8){transform:rotate(210deg);animation-delay:.583s}
.apple-spinner span:nth-child(9){transform:rotate(240deg);animation-delay:.666s}
.apple-spinner span:nth-child(10){transform:rotate(270deg);animation-delay:.75s}
.apple-spinner span:nth-child(11){transform:rotate(300deg);animation-delay:.833s}
.apple-spinner span:nth-child(12){transform:rotate(330deg);animation-delay:.916s}
@keyframes apple-fade{0%{opacity:1}100%{opacity:.2}}`,
	},
	"discord-blob": {
		name:     "Discord Blob",
		category: "Brand Inspired",
		html:     `<div class="loader discord-blob"></div>`,
		css: `.discord-blob{width:48px;height:48px;background:var(--primary);border-radius:33%;animation:discord-morph 2s ease-in-out infinite,spin 3s linear infinite}
@keyframes discord-morph{0%,100%{border-radius:33%}25%{border-radius:50% 33% 50% 33%}50%{border-radius:33% 50% 33% 50%}75%{border-radius:50% 33% 50% 33%}}`,
	},
	"instagram-gradient": {
		name:     "Instagram Gradient",
		category: "Brand Inspired",
		html:     `<div class="loader instagram-gradient"></div>`,
		css: `.instagram-gradient{width:48px;height:48px;border-radius:12px;background:linear-gradient(45deg,var(--primary),var(--secondary));animation:insta-pulse 1.5s ease-in-out infinite}
@keyframes insta-pulse{0%,100%{transform:scale(1);filter:hue-rotate(0deg)}50%{transform:scale(.9);filter:hue-rotate(30deg)}}`,
	},
	"facebook-dots": {
		name:     "Facebook Dots",
		category: "Brand Inspired",
		html:     `<div class="loader facebook-dots"><span></span><span></span><span></span></div>`,
		css: `.facebook-dots{display:flex;gap:6px}
.facebook-dots span{width:14px;height:14px;background:var(--primary);border-radius:50%;animation:fb-dots 1.4s ease-in-out infinite}
.facebook-dots span:nth-child(2){animation-delay:.2s}
.facebook-dots span:nth-child(3){animation-delay:.4s}
@keyframes fb-dots{0%,100%{transform:scale(1);opacity:1}50%{transform:scale(.6);opacity:.5}}`,
	},
	"youtube-play": {
		name:     "YouTube Play",
		category: "Brand Inspired",
		html:     `<div class="loader youtube-play"><span></span></div>`,
		css: `.youtube-play{position:relative;width:60px;height:42px;background:var(--primary);border-radius:8px;animation:yt-pulse 1s ease-in-out infinite}
.youtube-play span{position:absolute;top:50%;left:55%;transform:translate(-50%,-50%);width:0;height:0;border-left:18px solid #fff;border-top:11px solid transparent;border-bottom:11px solid transparent}
@keyframes yt-pulse{0%,100%{transform:scale(1)}50%{transform:scale(.95)}}`,
	},
	"slack-colors": {
		name:     "Slack Colors",
		category: "Brand Inspired",
		html:     `<div class="loader slack-colors"><span></span><span></span><span></span><span></span></div>`,
		css: `.slack-colors{position:relative;width:48px;height:48px;animation:spin 2s linear infinite}
.slack-colors span{position:absolute;width:12px;height:24px;border-radius:6px}
.slack-colors span:nth-child(1){background:var(--primary);top:0;left:50%;margin-left:-6px}
.slack-colors span:nth-child(2){background:var(--secondary);bottom:0;left:50%;margin-left:-6px}
.slack-colors span:nth-child(3){background:var(--primary);width:24px;height:12px;border-radius:6px;top:50%;left:0;margin-top:-6px}
.slack-colors span:nth-child(4){background:var(--secondary);width:24px;height:12px;border-radius:6px;top:50%;right:0;margin-top:-6px}
@keyframes spin{to{transform:rotate(360deg)}}`,
	},
}

// GetLoaders returns all available loader definitions grouped by category
func GetLoaders() []LoaderDef {
	result := make([]LoaderDef, 0, len(loaders))
	for id, data := range loaders {
		result = append(result, LoaderDef{
			ID:       id,
			Name:     data.name,
			Category: data.category,
		})
	}
	return result
}

// GetLoaderCSS returns the CSS for a specific loader, with color variables
func GetLoaderCSS(id string) string {
	if data, ok := loaders[id]; ok {
		return data.css
	}
	return ""
}

// GetLoaderHTML returns the HTML markup for a specific loader
func GetLoaderHTML(id string) string {
	if data, ok := loaders[id]; ok {
		return data.html
	}
	return ""
}
