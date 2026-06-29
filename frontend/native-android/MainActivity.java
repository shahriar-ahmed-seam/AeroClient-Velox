// MainActivity.java — registers AeroClient-Velox's custom VoltBridge Capacitor
// plugin with the WebView bridge.
//
// This is a TEMPLATE. The native android/ project is generated on demand
// (`npx cap add android`) and is not committed, so this file lives under
// frontend/native-android/ and OVERWRITES the generated MainActivity.java when
// the release workflow wires the project. See frontend/native-android/README.md.
//
// Capacitor 6 generates a MainActivity that merely extends BridgeActivity with
// no onCreate override, so a custom plugin is never registered. We override
// onCreate and call registerPlugin(VoltBridgePlugin.class) BEFORE
// super.onCreate(...), which is where Capacitor reads the registered plugins
// to build the bridge. Without this, JS calls fail with
// "VoltBridge plugin is not implemented on android".

package dev.volt.apiclient;

import android.os.Bundle;

import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {
    @Override
    public void onCreate(Bundle savedInstanceState) {
        registerPlugin(VoltBridgePlugin.class);
        super.onCreate(savedInstanceState);
    }
}
