#include <vector>
#include "engine.hpp"

namespace game {
class Entity {
public:
    int id;
    void update(float dt);
};

struct Vec2 { float x, y; };

int compute(int a) { return a; }
}
