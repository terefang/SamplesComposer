package glfw

/*
#include <stddef.h>
struct GLFWwindow;
struct GLFWmonitor;

// Dummy interface symbol so libwayland-client.so.0 is not required at runtime
const void *wl_callback_interface = NULL;
const void *wl_registry_interface = NULL;

int _glfwConnectWayland(int platformID, void *platform) {
    (void)platformID;
    (void)platform;
    return 0;
}

void* glfwGetWaylandDisplay(void) {
    return NULL;
}

void* glfwGetWaylandMonitor(struct GLFWmonitor *monitor) {
    (void)monitor;
    return NULL;
}

void* glfwGetWaylandWindow(struct GLFWwindow *window) {
    (void)window;
    return NULL;
}
*/
import "C"
