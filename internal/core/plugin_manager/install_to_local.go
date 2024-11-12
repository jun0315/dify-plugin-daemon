package plugin_manager

import (
	"time"

	"github.com/langgenius/dify-plugin-daemon/internal/types/entities/plugin_entities"
	"github.com/langgenius/dify-plugin-daemon/internal/utils/routine"
	"github.com/langgenius/dify-plugin-daemon/internal/utils/stream"
)

// InstallToLocal installs a plugin to local
func (p *PluginManager) InstallToLocal(
	plugin_unique_identifier plugin_entities.PluginUniqueIdentifier,
	source string,
	meta map[string]any,
) (
	*stream.Stream[PluginInstallResponse], error,
) {
	packageFile, err := p.packageBucket.Get(plugin_unique_identifier.String())
	if err != nil {
		return nil, err
	}

	err = p.installedBucket.Save(plugin_unique_identifier, packageFile)
	if err != nil {
		return nil, err
	}

	runtime, launchedChan, err := p.launchLocal(plugin_unique_identifier)
	if err != nil {
		return nil, err
	}

	response := stream.NewStream[PluginInstallResponse](128)
	routine.Submit(func() {
		defer response.Close()

		ticker := time.NewTicker(time.Second * 5) // check heartbeat every 5 seconds
		defer ticker.Stop()
		timer := time.NewTimer(time.Second * 240) // timeout after 240 seconds
		defer timer.Stop()

		for {
			select {
			case <-ticker.C:
				// heartbeat
				response.Write(PluginInstallResponse{
					Event: PluginInstallEventInfo,
					Data:  "Installing",
				})
			case <-timer.C:
				// timeout
				response.Write(PluginInstallResponse{
					Event: PluginInstallEventInfo,
					Data:  "Timeout",
				})
				runtime.Stop()
				return
			case <-launchedChan:
				// launched
				if err != nil {
					response.Write(PluginInstallResponse{
						Event: PluginInstallEventError,
						Data:  err.Error(),
					})
					runtime.Stop()
					return
				}
				response.Write(PluginInstallResponse{
					Event: PluginInstallEventDone,
					Data:  "Installed",
				})
				return
			}
		}

	})

	return response, nil
}
