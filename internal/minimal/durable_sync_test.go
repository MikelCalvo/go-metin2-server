package minimal

import (
	"os"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func TestMain(m *testing.M) {
	restoreAccountSync := accountstore.DisableDurableSyncForTest()
	restoreInteractionSync := interactionstore.DisableDurableSyncForTest()
	restoreItemSync := itemcatalog.DisableDurableSyncForTest()
	restoreLoginTicketSync := loginticket.DisableDurableSyncForTest()
	restoreQuestStateSync := queststate.DisableDurableSyncForTest()
	restoreStaticSync := staticstore.DisableDurableSyncForTest()

	code := m.Run()

	restoreStaticSync()
	restoreQuestStateSync()
	restoreLoginTicketSync()
	restoreItemSync()
	restoreInteractionSync()
	restoreAccountSync()
	os.Exit(code)
}
