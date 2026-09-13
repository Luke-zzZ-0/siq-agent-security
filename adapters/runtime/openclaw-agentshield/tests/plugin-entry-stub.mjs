// Minimal stand-in for "openclaw/plugin-sdk/plugin-entry": records the plugin
// spec on globalThis so tests can drive the registered hooks directly.
export function definePluginEntry(spec) {
  globalThis.__pluginEntry = spec;
  return spec;
}
