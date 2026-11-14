import { useEffect, useRef } from 'react'

const vertex = `attribute vec2 point; void main() { gl_Position = vec4(point, 0.0, 1.0); }`
const fragment = `
precision mediump float;
uniform vec2 resolution;
uniform vec2 pointer;
uniform float time, distortion, highlight, interaction, lightMode;
void main() {
  vec2 uv = gl_FragCoord.xy / resolution;
  vec2 p = uv * vec2(resolution.x / resolution.y, 1.0);
  float phase = time;
  vec2 warp = vec2(sin(p.y * 5.4 + cos(p.x * 3.5 + phase) + phase),
                  cos(p.x * 3.4 + sin(p.y * 7.1 - phase) - phase));
  p += distortion * .22 * warp;
  vec2 relative = (uv - pointer) * vec2(resolution.x / resolution.y, 1.0);
  float distanceToLight = length(relative);
  float viscousPull = exp(-distanceToLight * 8.0) * interaction;
  p += distortion * viscousPull * (relative * 1.3 + warp * .07);
  float folds = sin(8.1 * p.y + 4.3 * p.x + 2.3 * sin(3.1 * p.x - 1.9 * p.y + phase));
  float fine = cos(p.x * 9.0 + p.y * 11.0 + 2.0 * folds - phase);
  float steel = .15 + .30 * smoothstep(-.87, .83, folds)
              + highlight * .40 * pow(max(0.0, fine), 5.0);
  steel += viscousPull * (.22 + highlight * .22 * pow(max(0.0, fine), 3.0));
  vec3 dark = steel * vec3(.85, .93, 1.0);
  vec3 light = mix(vec3(.43, .48, .55), vec3(.99), steel);
  gl_FragColor = vec4(mix(dark, light, lightMode), 1.0);
}`

const qualityLevels = {
  low: { pixels: 110000, frame: 80 },
  balanced: { pixels: 260000, frame: 55 },
  high: { pixels: 480000, frame: 40 },
}

export function LiquidSurface({ quality, speed, distortion, highlight, mode }) {
  const ref = useRef(null)
  useEffect(() => {
    const canvas = ref.current
    const gl = canvas.getContext('webgl', { antialias: false, alpha: false, depth: false, powerPreference: 'low-power' })
    if (!gl) return
    const program = gl.createProgram()
    if (!program) return
    const shaders = [[gl.VERTEX_SHADER, vertex], [gl.FRAGMENT_SHADER, fragment]].map(([type, source]) => {
      const shader = gl.createShader(type)
      gl.shaderSource(shader, source)
      gl.compileShader(shader)
      gl.attachShader(program, shader)
      return shader
    })
    gl.linkProgram(program)
    for (const shader of shaders) gl.deleteShader(shader)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      gl.deleteProgram(program)
      return
    }
    gl.useProgram(program)
    const buffer = gl.createBuffer()
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1,-1, 3,-1, -1,3]), gl.STATIC_DRAW)
    const point = gl.getAttribLocation(program, 'point')
    gl.enableVertexAttribArray(point)
    gl.vertexAttribPointer(point, 2, gl.FLOAT, false, 0, 0)
    const uniforms = Object.fromEntries(['resolution','pointer','time','distortion','highlight','interaction','lightMode'].map((name) => [name, gl.getUniformLocation(program, name)]))
    gl.uniform1f(uniforms.distortion, distortion / 100)
    gl.uniform1f(uniforms.highlight, highlight / 100)
    gl.uniform1f(uniforms.lightMode, mode === 'light' ? 1 : 0)

    const level = qualityLevels[quality] || qualityLevels.balanced
    let timer, frame, last, lastPointer = -Infinity, phase = 0
    let x = .5, y = .5, targetX = .5, targetY = .5
    function resize() {
      const { clientWidth: width, clientHeight: height } = canvas
      const scale = Math.min(1, Math.sqrt(level.pixels / Math.max(1, width * height)))
      canvas.width = Math.max(1, Math.round(width * scale))
      canvas.height = Math.max(1, Math.round(height * scale))
      gl.viewport(0, 0, canvas.width, canvas.height)
      gl.uniform2f(uniforms.resolution, canvas.width, canvas.height)
    }
    function render(now) {
      if (document.hidden || gl.isContextLost()) return
      if (document.querySelector('.dialog .runtime-log')) {
        last = undefined
        timer = window.setTimeout(() => { frame = requestAnimationFrame(render) }, 250)
        return
      }
      phase = (phase + Math.min(now - (last ?? now), 180) * (.00008 + speed * .000005)) % (Math.PI * 2)
      last = now
      x += (targetX - x) * .2
      y += (targetY - y) * .2
      const activity = Math.max(0, 1 - (now - lastPointer) / 1800)
      gl.uniform1f(uniforms.time, phase)
      gl.uniform1f(uniforms.interaction, activity)
      gl.uniform2f(uniforms.pointer, x, y)
      gl.drawArrays(gl.TRIANGLES, 0, 3)
      if (!canvas.dataset.ready) canvas.dataset.ready = 'true'
      timer = window.setTimeout(() => { frame = requestAnimationFrame(render) }, activity > 0 ? level.frame : 125)
    }
    function pointerMove(event) {
      targetX = event.clientX / window.innerWidth
      targetY = 1 - event.clientY / window.innerHeight
      lastPointer = performance.now()
    }
    function stop() {
      window.clearTimeout(timer)
      cancelAnimationFrame(frame)
      last = undefined
    }
    function visibility() {
      stop()
      if (!document.hidden && !gl.isContextLost()) frame = requestAnimationFrame(render)
    }
    function lost(event) {
      event.preventDefault()
      stop()
      delete canvas.dataset.ready
    }
    const observer = new ResizeObserver(resize)
    observer.observe(canvas)
    resize()
    visibility()
    document.addEventListener('visibilitychange', visibility)
    window.addEventListener('pointermove', pointerMove, { passive: true })
    canvas.addEventListener('webglcontextlost', lost)
    return () => {
      stop()
      observer.disconnect()
      document.removeEventListener('visibilitychange', visibility)
      window.removeEventListener('pointermove', pointerMove)
      canvas.removeEventListener('webglcontextlost', lost)
      delete canvas.dataset.ready
      gl.deleteBuffer(buffer)
      gl.deleteProgram(program)
    }
  }, [quality, speed, distortion, highlight, mode])
  return <canvas ref={ref} className="liquid-canvas" />
}
