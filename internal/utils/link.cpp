#include <Windows.h>
#include <WinNls.h>
#include <ShObjIdl.h>
#include <ShlGuid.h>
#include <memory>

extern "C"
HRESULT CreateLink(wchar_t* lpszPathObj, wchar_t* lpszPathLink)
{
    HRESULT hres;
    hres = CoInitializeEx(0, COINIT_MULTITHREADED | COINIT_SPEED_OVER_MEMORY);
    if (FAILED(hres)) {
        return hres;
    }
    std::shared_ptr<int> couninit(nullptr, [](int*){
        CoUninitialize();
    });

    IShellLink* psl;

    // Get a pointer to the IShellLink interface. It is assumed that CoInitialize
    // has already been called.
    hres = CoCreateInstance(CLSID_ShellLink, NULL, CLSCTX_INPROC_SERVER, IID_IShellLink, (LPVOID*)&psl);
    if (SUCCEEDED(hres))
    {
        IPersistFile* ppf;

        // Set the path to the shortcut target and add the description. 
        psl->SetPath(lpszPathObj);
        //psl->SetDescription(lpszDesc);

        // Query IShellLink for the IPersistFile interface, used for saving the 
        // shortcut in persistent storage. 
        hres = psl->QueryInterface(IID_IPersistFile, (LPVOID*)&ppf);

        if (SUCCEEDED(hres))
        {
            //WCHAR wsz[MAX_PATH];

            // Ensure that the string is Unicode. 
           // MultiByteToWideChar(CP_ACP, 0, lpszPathLink, -1, wsz, MAX_PATH);

            // Add code here to check return value from MultiByteWideChar 
            // for success.

            // Save the link by calling IPersistFile::Save. 
            hres = ppf->Save(lpszPathLink, TRUE);
            ppf->Release();
        }
        psl->Release();
    }
    return hres;
}

extern "C" int msgbox(wchar_t* msg) {
    return MessageBoxW(NULL, msg, L"cap", MB_OK);
}

// int main() {
//     return 0;
// }