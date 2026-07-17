// Enter only after every fragment has initialized its top-level constants.
// Function declarations are hoisted across the combined script, but const
// bindings in the security and taint fragments remain in the temporal dead
// zone until evaluation reaches them.
main();
