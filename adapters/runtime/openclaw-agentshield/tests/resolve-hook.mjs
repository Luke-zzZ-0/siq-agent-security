// Module resolution hook: map the OpenClaw SDK entry to the local stub.
const stubURL = new URL("./plugin-entry-stub.mjs", import.meta.url).href;

export async function resolve(specifier, context, next) {
  if (specifier === "openclaw/plugin-sdk/plugin-entry") {
    return { url: stubURL, shortCircuit: true };
  }
  return next(specifier, context);
}
