const std = @import("std");
const mem = std.mem;
const config = @import("config.zig");

pub const Point = struct {
    x: f32,
    y: f32,

    pub fn len(self: Point) f32 {
        return @sqrt(self.x * self.x + self.y * self.y);
    }

    fn hidden(self: Point) void {
        _ = self;
    }
};

pub const Color = enum { red, green, blue };

pub const Shape = union(enum) {
    circle: f32,
    square: f32,
};

pub const max_items: usize = 64;

pub fn add(a: i32, b: i32) i32 {
    return a + b;
}

fn privateHelper() void {}

pub fn main() !void {
    std.debug.print("hi\n", .{});
}
