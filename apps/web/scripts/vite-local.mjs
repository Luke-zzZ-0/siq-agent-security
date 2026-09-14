// Set the local app mode inside Node so npm works with both Windows and POSIX shells.
// Run the locked Vite CLI in this process to preserve its arguments and exit behavior.
process.env.VITE_APP = 'agentshield';
await import('../node_modules/vite/bin/vite.js');
