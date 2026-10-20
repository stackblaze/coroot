package auditor

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
)

func TestNodeConsumersVisibility(t *testing.T) {
	own := model.NewApplication(model.NewApplicationId("", "team-a", model.ApplicationKindDeployment, "web"))
	other := model.NewApplication(model.NewApplicationId("", "team-b", model.ApplicationKindDeployment, "db"))
	another := model.NewApplication(model.NewApplicationId("", "team-c", model.ApplicationKindDeployment, "cache"))
	node := model.NewNode("", model.NewNodeId("m1", ""))
	for _, app := range []*model.Application{own, other, another} {
		i := model.NewInstance(app.Id.Name+"-1", app)
		i.GetOrCreateContainer(app.Id.Name, app.Id.Name)
		node.Instances = append(node.Instances, i)
	}

	all := getNodeConsumers(node, nil)
	assert.Len(t, all.cpu, 3)
	assert.Contains(t, all.cpu, "db")

	visible := getNodeConsumers(node, func(app *model.Application) bool { return app == own })
	assert.Len(t, visible.cpu, 2)
	assert.Contains(t, visible.cpu, "web")
	assert.Contains(t, visible.cpu, otherConsumers)
	assert.NotContains(t, visible.cpu, "db")
	assert.NotContains(t, visible.memory, "cache")
}
