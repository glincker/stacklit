module Data.Shape (Shape(..), area, mkSquare) where

import Data.List (sortBy)
import qualified Data.Map as Map
import Control.Monad

data Shape = Circle Double | Rect Double Double

newtype Wrapper a = Wrapper { unwrap :: a }

type Name = String

class Describable a where
  describe :: a -> String

instance Describable Shape where
  describe _ = "shape"

area :: Shape -> Double
area (Circle r) = pi * r * r
area (Rect w h) = w * h

mkSquare :: Double -> Shape
mkSquare s = Rect s s
