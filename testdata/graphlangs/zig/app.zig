const base = @import("base.zig");

pub fn run(b: *base.Base) void {
    _ = base.helper(1);
    b.greet();
}
