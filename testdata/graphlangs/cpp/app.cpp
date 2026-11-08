#include "base.hpp"

class App : public Base {
public:
    void run(Base* b) {
        greet();
        b->greet();
    }
};
