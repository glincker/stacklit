library(dplyr)
require(ggplot2)
source("helpers.R")

add <- function(a, b = 2) {
  a + b
}

.hidden <- function(x) x

compute = function(x, ...) {
  x
}

PI_VALUE <- 3.14
