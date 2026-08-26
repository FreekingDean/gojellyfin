module.exports = async function globalTeardown() {
  if (globalThis.__BROWSER__) {
    await globalThis.__BROWSER__.close();
  }
  if (globalThis.__APP__) {
    await globalThis.__APP__.stop();
  }
};
