export function Brand({ className = '' }) {
  return <div className={`brand ${className}`.trim()}><span className="brand-mark" aria-hidden="true" /><span>SILICON</span></div>
}
