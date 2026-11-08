import Foundation
import UIKit

public protocol Drawable {
    func draw()
}

public struct Point {
    public var x: Int
    public func length() -> Double { return 0 }
}

open class Canvas: Drawable {
    public init() {}
    public func draw() {}
    private func hidden() {}
}

public enum Direction { case north, south }

public func render(_ c: Canvas, scale: Int) -> Bool { return true }

func internalHelper() {}
