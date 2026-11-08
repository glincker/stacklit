/// A helper.
pub fn helper(x: i32) i32 {
    return x;
}

pub const Base = struct {
    pub fn greet(self: *Base) void {
        _ = self;
    }
};
