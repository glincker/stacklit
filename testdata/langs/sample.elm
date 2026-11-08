module Main exposing (main, add, Model)

import Html exposing (Html, div, text)
import Json.Decode as Decode

type alias Model =
    { count : Int }

type Msg
    = Increment
    | Decrement

add : Int -> Int -> Int
add a b =
    a + b

helper : Int -> Int
helper x =
    x

main : Html msg
main =
    div [] [ text "hi" ]
